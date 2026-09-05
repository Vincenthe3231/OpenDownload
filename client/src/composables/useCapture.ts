import {
  ListCapturedStreams,
  QueueCapturedStream,
  StartFirefoxCapture,
  StopFirefoxCapture,
} from '../../wailsjs/wailsjs/go/main/App';

export const useCapture = () => {
  const startCapture = () => StartFirefoxCapture();
  const stopCapture = () => StopFirefoxCapture();
  const refreshStreams = () => ListCapturedStreams();
  const downloadCaptured = (jobID: string, capturedStreamID: string, path: string) => QueueCapturedStream(jobID, capturedStreamID, path);

  return { startCapture, stopCapture, refreshStreams, downloadCaptured };
};
