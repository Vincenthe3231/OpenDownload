import {
  DownloadCapturedStream,
  ListCapturedStreams,
  StartFirefoxCapture,
  StopFirefoxCapture,
} from '../../wailsjs/wailsjs/go/main/App';

export const useCapture = () => {
  const startCapture = () => StartFirefoxCapture();
  const stopCapture = () => StopFirefoxCapture();
  const refreshStreams = () => ListCapturedStreams();
  const downloadCaptured = (id: string, path: string) => DownloadCapturedStream(id, path);

  return { startCapture, stopCapture, refreshStreams, downloadCaptured };
};
