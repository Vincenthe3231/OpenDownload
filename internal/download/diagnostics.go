package download

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/opendownload/opendownload/internal/diagnostics"
	"github.com/opendownload/opendownload/internal/downloader"
	"github.com/opendownload/opendownload/internal/media"
	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/transport"
)

const (
	downloadAccessForbiddenCode diagnostics.Code = "DOWNLOAD_ACCESS_FORBIDDEN"
	downloadRateLimitedCode     diagnostics.Code = "DOWNLOAD_RATE_LIMITED"
)

type failureClassification struct {
	code       diagnostics.Code
	message    string
	retryable  bool
	httpStatus int
}

func (service *Service) diagnosticForFailure(request Request, kind media.SourceKind, err error) *diagnostics.Diagnostic {
	classification := classifyDownloadFailure(kind, err)
	diagnostic := diagnostics.New(
		classification.code,
		diagnostics.StageDownload,
		classification.retryable,
		classification.message,
		diagnostics.NewID(),
		time.Now().UTC(),
	)
	contextValues := map[string]string{"operationId": request.ID}
	if classification.httpStatus != 0 {
		contextValues["httpStatus"] = strconv.Itoa(classification.httpStatus)
	}
	diagnostic = diagnostic.WithContext(contextValues)
	service.recordTechnicalFailure(request, kind, err, diagnostic, classification.httpStatus)
	return &diagnostic
}

func classifyDownloadFailure(kind media.SourceKind, err error) failureClassification {
	if errors.Is(err, context.Canceled) {
		return failureClassification{
			code:      diagnostics.DownloadCancelled,
			message:   "Download was cancelled.",
			retryable: true,
		}
	}

	if statusCode := httpStatusFromError(err); statusCode != 0 {
		switch statusCode {
		case 401:
			return failureClassification{
				code:       diagnostics.DownloadAuthRequired,
				message:    "Authentication is required to download this source.",
				httpStatus: statusCode,
			}
		case 403:
			return failureClassification{
				code:       downloadAccessForbiddenCode,
				message:    "Access to this source is forbidden.",
				httpStatus: statusCode,
			}
		case 404:
			return failureClassification{
				code:       diagnostics.DownloadSourceNotFound,
				message:    "The download source was not found.",
				httpStatus: statusCode,
			}
		case 429:
			return failureClassification{
				code:       downloadRateLimitedCode,
				message:    "The source is rate limited. Try again later.",
				retryable:  true,
				httpStatus: statusCode,
			}
		default:
			return failureClassification{
				code:       diagnostics.DownloadHTTPFailed,
				message:    "The source returned an HTTP error.",
				retryable:  statusCode >= 500 || statusCode == 408,
				httpStatus: statusCode,
			}
		}
	}

	if isManifestFailure(kind, err) {
		return failureClassification{
			code:    diagnostics.DownloadManifestInvalid,
			message: "The media manifest is invalid.",
		}
	}
	if isFilesystemFailure(err) {
		return failureClassification{
			code:    diagnostics.DownloadWriteFailed,
			message: "OpenDownload could not write the download file.",
		}
	}
	if isSegmentFailure(err) {
		return failureClassification{
			code:      diagnostics.DownloadSegmentFailed,
			message:   "A media segment failed to download.",
			retryable: true,
		}
	}
	if isNetworkFailure(err) {
		return failureClassification{
			code:      diagnostics.DownloadTransportFailed,
			message:   "The network request failed.",
			retryable: true,
		}
	}
	return failureClassification{
		code:    diagnostics.DownloadHTTPFailed,
		message: "The download failed.",
	}
}

func isManifestFailure(kind media.SourceKind, err error) bool {
	var parserErr *parser.ValidationError
	if errors.As(err, &parserErr) {
		return true
	}
	if kind != media.SourceHLS && kind != media.SourceDASH {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "playlist") || strings.Contains(message, "manifest") || strings.Contains(message, "format was not found")
}

func isSegmentFailure(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "segment ")
}

func httpStatusFromError(err error) int {
	var statusErr *transport.HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode
	}

	var validationErr *downloader.ValidationError
	if !errors.As(err, &validationErr) {
		return 0
	}
	fields := strings.Fields(strings.ToLower(validationErr.Detail))
	for index, field := range fields {
		if field != "received" || index+1 >= len(fields) {
			continue
		}
		status, parseErr := strconv.Atoi(strings.Trim(fields[index+1], ".,:;"))
		if parseErr == nil && status >= 100 && status <= 599 {
			return status
		}
	}
	return 0
}

func isFilesystemFailure(err error) bool {
	var pathErr *os.PathError
	var linkErr *os.LinkError
	var syscallErr *os.SyscallError
	var conflictErr *ConflictError
	return errors.As(err, &pathErr) ||
		errors.As(err, &linkErr) ||
		errors.As(err, &syscallErr) ||
		errors.As(err, &conflictErr) ||
		errors.Is(err, os.ErrPermission) ||
		errors.Is(err, os.ErrExist)
}

func isNetworkFailure(err error) bool {
	var transportErr *downloader.TransportError
	var networkErr net.Error
	return errors.As(err, &transportErr) || errors.As(err, &networkErr) || errors.Is(err, context.DeadlineExceeded)
}

func (service *Service) recordTechnicalFailure(request Request, kind media.SourceKind, err error, diagnostic diagnostics.Diagnostic, httpStatus int) {
	if service.technicalStore == nil || !service.technicalStore.Enabled() {
		return
	}

	technical := technicalFailureContext(request, kind, err, httpStatus)
	if request.TechnicalContext != nil {
		if captured := request.TechnicalContext(); captured != nil {
			technical = *captured
			technical.RawError = combineRawErrors(technical.RawError, err.Error())
			if technical.RequestID == "" {
				technical.RequestID = request.ID
			}
			if technical.RequestMethod == "" {
				technical.RequestMethod = "GET"
			}
			if technical.RequestType == "" {
				technical.RequestType = string(kind)
			}
			if technical.RequestURL == "" {
				technical.RequestURL = request.URL
			}
			if len(technical.RequestHeaders) == 0 {
				technical.RequestHeaders = cloneHeaders(request.Headers)
			}
		}
	}
	if httpStatus != 0 {
		technical.HTTPStatus = httpStatus
	}
	technical.DiagnosticID = diagnostic.DiagnosticID
	technical.OccurredAt = diagnostic.OccurredAt
	service.technicalStore.Record(technical)
}

func technicalFailureContext(request Request, kind media.SourceKind, err error, httpStatus int) diagnostics.TechnicalDiagnostic {
	return diagnostics.TechnicalDiagnostic{
		RawError:       err.Error(),
		RequestID:      request.ID,
		RequestMethod:  "GET",
		RequestType:    string(kind),
		RequestURL:     request.URL,
		RequestHeaders: cloneHeaders(request.Headers),
		HTTPStatus:     httpStatus,
	}
}

func combineRawErrors(captureError, downloadError string) string {
	captureError = strings.TrimSpace(captureError)
	downloadError = strings.TrimSpace(downloadError)
	if captureError == "" {
		return downloadError
	}
	if downloadError == "" {
		return captureError
	}
	return captureError + "\nDownload failure: " + downloadError
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	copyHeaders := make(map[string]string, len(headers))
	for key, value := range headers {
		copyHeaders[key] = value
	}
	return copyHeaders
}

func cloneDiagnostic(diagnostic *diagnostics.Diagnostic) *diagnostics.Diagnostic {
	if diagnostic == nil {
		return nil
	}
	copyDiagnostic := *diagnostic
	if len(diagnostic.SafeContext) > 0 {
		copyDiagnostic.SafeContext = make(map[string]string, len(diagnostic.SafeContext))
		for key, value := range diagnostic.SafeContext {
			copyDiagnostic.SafeContext[key] = value
		}
	}
	return &copyDiagnostic
}
