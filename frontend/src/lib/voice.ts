// Push-to-talk audio capture for chat voice input (WebView2-compatible).
// Web Speech API is NOT available in WebView2; we record audio and send it
// to the backend for transcription.

export interface VoiceRecorder {
  stop: () => Promise<{ base64: string; mime: string }>;
  cancel: () => void;
}

/** Starts microphone recording. Resolves once recording has actually begun. */
export async function startRecording(): Promise<VoiceRecorder> {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  const mime = pickMime();
  const rec = new MediaRecorder(stream, mime ? { mimeType: mime } : undefined);
  const chunks: Blob[] = [];
  rec.ondataavailable = (e) => { if (e.data && e.data.size > 0) chunks.push(e.data); };
  rec.start();

  const cleanup = () => stream.getTracks().forEach((t) => t.stop());

  return {
    stop: () =>
      new Promise((resolve, reject) => {
        rec.onstop = async () => {
          cleanup();
          try {
            const blob = new Blob(chunks, { type: rec.mimeType || 'audio/webm' });
            const base64 = await blobToBase64(blob);
            resolve({ base64, mime: blob.type });
          } catch (e) { reject(e); }
        };
        rec.stop();
      }),
    cancel: () => { try { rec.stop(); } catch {} cleanup(); },
  };
}

function pickMime(): string {
  const candidates = ['audio/webm;codecs=opus', 'audio/webm', 'audio/ogg;codecs=opus', 'audio/mp4'];
  for (const c of candidates) {
    if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(c)) return c;
  }
  return '';
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onloadend = () => {
      const s = String(r.result || '');
      const comma = s.indexOf(',');
      resolve(comma >= 0 ? s.slice(comma + 1) : s); // strip data: prefix
    };
    r.onerror = () => reject(r.error);
    r.readAsDataURL(blob);
  });
}
