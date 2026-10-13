import { useEffect, useRef, useState } from 'react';
import { useAtomValue, useSetAtom } from 'jotai';

import { getBaseUrl } from '@/lib/service';
import { microphoneEnabledAtom } from '@/jotai/audio';
import { videoVolumeAtom } from '@/jotai/screen';

// One audio session survives video transport changes and owns microphone input.
export const WebrtcAudio = () => {
  const volume = useAtomValue(videoVolumeAtom);
  const microphone = useAtomValue(microphoneEnabledAtom);
  const setMicrophone = useSetAtom(microphoneEnabledAtom);
  const elementRef = useRef<HTMLAudioElement>(null);
  const [attempt, setAttempt] = useState(0);
  const [sender, setSender] = useState<RTCRtpSender | null>(null);
  useEffect(() => {
    if (!window.RTCPeerConnection) return;
    const ws = new WebSocket(`${getBaseUrl('ws')}/api/stream/audio/webrtc`);
    const peer = new RTCPeerConnection({ iceServers: [] });
    let disposed = false;
    let retry: ReturnType<typeof setTimeout> | undefined;
    const candidates: RTCIceCandidateInit[] = [];
    const reconnect = () => {
      if (!disposed && !retry) retry = setTimeout(() => setAttempt((value) => value + 1), 3000);
    };
    const send = (event: string, value: unknown) => {
      if (ws.readyState === WebSocket.OPEN)
        ws.send(JSON.stringify({ event, data: JSON.stringify(value) }));
    };
    peer.onicecandidate = ({ candidate }) => {
      if (candidate) send('audio-candidate', candidate);
    };
    peer.onconnectionstatechange = () => {
      if (peer.connectionState === 'failed') reconnect();
    };
    peer.ontrack = ({ track }) => {
      if (elementRef.current) elementRef.current.srcObject = new MediaStream([track]);
    };
    ws.onopen = async () => {
      if (disposed) {
        ws.close();
        return;
      }
      const transceiver = peer.addTransceiver('audio', { direction: 'sendrecv' });
      setSender(transceiver.sender);
      try {
        await peer.setLocalDescription(await peer.createOffer());
        if (!disposed) send('audio-offer', peer.localDescription);
      } catch {
        reconnect();
      }
    };
    ws.onmessage = async ({ data }) => {
      if (disposed) return;
      try {
        const msg = JSON.parse(data);
        const value = msg.data && JSON.parse(msg.data);
        if (msg.event === 'audio-answer') {
          await peer.setRemoteDescription(value);
          for (const candidate of candidates.splice(0)) await peer.addIceCandidate(candidate);
        } else if (msg.event === 'audio-candidate' && value?.candidate) {
          if (peer.remoteDescription) await peer.addIceCandidate(value);
          else candidates.push(value);
        }
      } catch (error) {
        if (!disposed) {
          console.error('Audio signaling failed', error);
          reconnect();
        }
      }
    };
    ws.onerror = reconnect;
    ws.onclose = reconnect;
    const heartbeat = setInterval(() => send('heartbeat', ''), 10000);
    const timeout = setTimeout(() => {
      if (peer.connectionState !== 'connected') reconnect();
    }, 10000);
    return () => {
      disposed = true;
      clearInterval(heartbeat);
      clearTimeout(timeout);
      clearTimeout(retry);
      ws.close();
      peer.close();
      setSender(null);
    };
  }, [attempt]);
  useEffect(() => {
    if (!sender) return;
    let disposed = false;
    let input: MediaStream | undefined;
    if (microphone) {
      if (!navigator.mediaDevices?.getUserMedia) {
        setMicrophone(false);
        return;
      }
      navigator.mediaDevices
        .getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true } })
        .then(async (stream) => {
          input = stream;
          if (disposed) {
            stream.getTracks().forEach((track) => track.stop());
            return;
          }
          await sender.replaceTrack(stream.getAudioTracks()[0]);
        })
        .catch((error) => {
          if (!disposed) {
            console.error('Microphone failed', error);
            setMicrophone(false);
          }
        });
    } else {
      void sender.replaceTrack(null).catch(console.error);
    }
    return () => {
      disposed = true;
      input?.getTracks().forEach((track) => track.stop());
      void sender.replaceTrack(null).catch(() => undefined);
    };
  }, [microphone, sender, setMicrophone]);
  useEffect(() => {
    const element = elementRef.current;
    if (!element) return;
    element.volume = volume / 100;
    element.muted = volume === 0;
    if (volume > 0) void element.play().catch(console.error);
  }, [volume, sender]);
  return <audio ref={elementRef} autoPlay playsInline />;
};
