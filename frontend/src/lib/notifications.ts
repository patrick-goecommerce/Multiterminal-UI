import * as App from '../../wailsjs/go/backend/App';

/** Send a native OS notification via the Go backend. */
export function sendNotification(title: string, body: string) {
  App.SendNotification(title, body);
}
