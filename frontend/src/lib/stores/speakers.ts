import { writable } from 'svelte/store';

export interface Speaker {
	ip: string;
	name: string;
	model: string;
	active: boolean;
	firmware?: string;
}

export const speakers = writable<Speaker[]>([]);
export const activeSpeaker = writable<Speaker | null>(null);

export function updateSpeakers(speakerList: Speaker[]) {
	speakers.set(speakerList);
	const active = speakerList.find((s) => s.active);
	if (active) {
		activeSpeaker.set(active);
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
