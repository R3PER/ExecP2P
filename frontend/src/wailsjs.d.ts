import * as Bridge from './wailsjs/go/wailsbridge/Bridge';

declare global {
  interface Window {
    go: {
      wailsbridge: {
        Bridge: typeof Bridge;
      }
    };
    runtime: {
      EventsOn: (eventName: string, callback: (data: any) => void) => void;
      EventsOff: (eventName: string) => void;
      EventsEmit: (eventName: string, data?: any) => void;
    };
  }
}

export {};
