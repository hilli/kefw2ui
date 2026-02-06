import { writable } from 'svelte/store';

export interface Speaker {
	ip: string;
	name: string;
	model: string;
	active: boolean;
	isDefault?: boolean;
	firmware?: string;
}

export const speakers = writable<Speaker[]>([]);
export const activeSpeaker = writable<Speaker | null>(null);
export const defaultSpeakerIP = writable<string>('');

export function updateSpeakers(speakerList: Speaker[], defaultIP?: string) {
	speakers.set(speakerList);
	const active = speakerList.find((s) => s.active);
	if (active) {
		activeSpeaker.set(active);
	}
	if (defaultIP !== undefined) {
		defaultSpeakerIP.set(defaultIP);
	}
}

export function setActiveSpeaker(speaker: Speaker) {
	activeSpeaker.set(speaker);
	speakers.update((list) =>
		list.map((s) => ({
			...s,
			active: s.ip === speaker.ip
		}))
	);
}

export function setDefaultSpeakerIP(ip: string) {
	defaultSpeakerIP.set(ip);
	speakers.update((list) =>
		list.map((s) => ({
			...s,
			isDefault: s.ip === ip
		}))
	);
}
