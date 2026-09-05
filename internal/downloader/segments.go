package downloader

import (
	"context"
	"fmt"
	"sync"
)

type segmentResult struct {
	index int
	data  []byte
	err   error
}

// downloadOrderedSegments keeps at most two worker windows of segment data in
// memory while preserving source order in the caller supplied work output.
func downloadOrderedSegments(ctx context.Context, total, workers int, outPath string, fetch func(context.Context, int) ([]byte, error), downloaded func(int)) error {
	if workers <= 0 {
		workers = 1
	}

	file, err := createWorkOutput(outPath)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = file.Close()
			removeWorkOutput(outPath)
		}
	}()

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	results := make(chan segmentResult, workers)
	var workerWG sync.WaitGroup
	for range workers {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for index := range jobs {
				data, fetchErr := fetch(childCtx, index)
				result := segmentResult{index: index, data: data, err: fetchErr}
				select {
				case results <- result:
				case <-childCtx.Done():
					return
				}
			}
		}()
	}

	window := workers * 2
	nextDispatch, nextWrite, inFlight := 0, 0, 0
	pending := make(map[int][]byte, window)
	for nextWrite < total {
		var jobChannel chan<- int
		if nextDispatch < total && inFlight < window {
			jobChannel = jobs
		}

		select {
		case jobChannel <- nextDispatch:
			nextDispatch++
			inFlight++
		case result := <-results:
			inFlight--
			if result.err != nil {
				cancel()
				close(jobs)
				workerWG.Wait()
				return result.err
			}
			pending[result.index] = result.data
			if downloaded != nil {
				downloaded(len(result.data))
			}
			for {
				data, exists := pending[nextWrite]
				if !exists {
					break
				}
				if _, writeErr := file.Write(data); writeErr != nil {
					cancel()
					close(jobs)
					workerWG.Wait()
					return fmt.Errorf("write segment %d: %w", nextWrite, writeErr)
				}
				delete(pending, nextWrite)
				nextWrite++
			}
		case <-ctx.Done():
			cancel()
			close(jobs)
			workerWG.Wait()
			return ctx.Err()
		}
	}

	close(jobs)
	workerWG.Wait()
	if err := file.Close(); err != nil {
		return fmt.Errorf("close work output: %w", err)
	}
	succeeded = true
	return nil
}
