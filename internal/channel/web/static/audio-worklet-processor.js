// AudioWorklet capture: downsample to 16 kHz mono int16 (Deepgram live linear16).
class MavenPcmCaptureProcessor extends AudioWorkletProcessor {
  process(inputs) {
    const input = inputs[0];
    if (!input || input.length === 0) {
      return true;
    }
    const channel = input[0];
    if (!channel || channel.length === 0) {
      return true;
    }
    const inRate = globalThis.sampleRate;
    const outRate = 16000;
    const ratio = inRate / outRate;
    const outLen = Math.floor(channel.length / ratio);
    if (outLen <= 0) {
      return true;
    }
    const out = new Int16Array(outLen);
    const last = channel.length - 1;
    for (let i = 0; i < outLen; i++) {
      const pos = i * ratio;
      const idx = Math.floor(pos);
      const frac = pos - idx;
      const a = channel[idx] ?? 0;
      const b = channel[Math.min(idx + 1, last)] ?? 0;
      const x = a + (b - a) * frac;
      const clamped = Math.max(-1, Math.min(1, x));
      out[i] = clamped < 0 ? clamped * 0x8000 : clamped * 0x7fff;
    }
    this.port.postMessage(out.buffer, [out.buffer]);
    return true;
  }
}
registerProcessor("maven-pcm-capture", MavenPcmCaptureProcessor);
