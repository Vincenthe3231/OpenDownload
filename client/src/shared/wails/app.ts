import * as generatedApp from '../../../wailsjs/wailsjs/go/main/App';

// The generated Wails module changes whenever the Go bridge changes. Feature
// adapters use this narrow escape hatch so generated types do not leak into UI
// state or components.
export function getAppBridge<TBridge>(): TBridge {
  return generatedApp as unknown as TBridge;
}
