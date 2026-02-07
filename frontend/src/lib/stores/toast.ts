import { writable } from 'svelte/store';

export type ToastType = 'error' | 'warning' | 'success' | 'info';

export interface Toast {
	id: number;
	message: string;
	type: ToastType;
	timestamp: number;
}

let nextId = 0;

function createToastStore() {
	const { subscribe, update } = writable<Toast[]>([]);

	function addToast(message: string, type: ToastType = 'error', duration = 4000) {
		const id = nextId++;
		const toast: Toast = {
			id,
			message,
			type,
			timestamp: Date.now()
		};

		update((toasts) => [...toasts, toast]);

		// Auto-remove after duration
		setTimeout(() => {
			removeToast(id);
		}, duration);

		return id;
	}

	function removeToast(id: number) {
		update((toasts) => toasts.filter((t) => t.id !== id));
	}

	return {
		subscribe,
		addToast,
		removeToast,
		error: (message: string) => addToast(message, 'error'),
		warning: (message: string) => addToast(message, 'warning'),
		success: (message: string) => addToast(message, 'success', 3000),
		info: (message: string) => addToast(message, 'info', 3000)
	};
}

export const toasts = createToastStore();
