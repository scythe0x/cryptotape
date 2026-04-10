import { useCallback, useEffect, useRef, useState } from "react";

const SOUND_FILE_PATH = "door-chime.wav";
const STORAGE_KEY = "cryptotape:door-chime";

function readInitialMutedState() {
	if (typeof window === "undefined") return false;
	return window.localStorage.getItem(STORAGE_KEY) === "1";
}

export function useAlertSound() {
	const [muted, setMuted] = useState(readInitialMutedState);
	const audioContextRef = useRef<AudioContext | null>(null);
	const audioBufferRef = useRef<AudioBuffer | null>(null);

	const ensureAudioContext = useCallback(async () => {
		const AudioContextCtor = window.AudioContext;
		if (!AudioContextCtor) return null;

		if (!audioContextRef.current) {
			audioContextRef.current = new AudioContextCtor();
		}

		if (audioContextRef.current.state === "suspended") {
			await audioContextRef.current.resume();
		}

		return audioContextRef.current;
	}, []);

	const loadAudioBuffer = useCallback(async (ctx: AudioContext) => {
		if (audioBufferRef.current) return audioBufferRef.current;

		try {
			const response = await fetch(SOUND_FILE_PATH);
			const arrayBuffer = await response.arrayBuffer();
			const audioBuffer = await ctx.decodeAudioData(arrayBuffer);
			audioBufferRef.current = audioBuffer;
			return audioBuffer;
		} catch (error) {
			console.error("Failed to load alert sound:", error);
			return null;
		}
	}, []);

	const playNewAlert = useCallback(async () => {
		if (muted) return;

		try {
			const ctx = await ensureAudioContext();
			if (!ctx) return;

			const audioBuffer = await loadAudioBuffer(ctx);
			if (!audioBuffer) return;

			const source = ctx.createBufferSource();
			source.buffer = audioBuffer;
			source.connect(ctx.destination);
			source.start(0);
		} catch {
			// Browsers can block autoplay until the user interacts with the page.
		}
	}, [ensureAudioContext, muted, loadAudioBuffer]);

	const toggleMuted = useCallback(() => {
		setMuted((prev) => !prev);
	}, []);

	useEffect(() => {
		window.localStorage.setItem(STORAGE_KEY, muted ? "1" : "0");
	}, [muted]);

	useEffect(() => {
		if (muted) return;
		void ensureAudioContext();
	}, [ensureAudioContext, muted]);

	useEffect(() => {
		const unlockAudio = () => {
			void ensureAudioContext();
		};

		window.addEventListener("pointerdown", unlockAudio, { passive: true });
		window.addEventListener("keydown", unlockAudio);

		return () => {
			window.removeEventListener("pointerdown", unlockAudio);
			window.removeEventListener("keydown", unlockAudio);
		};
	}, [ensureAudioContext]);

	return { muted, toggleMuted, playNewAlert };
}
