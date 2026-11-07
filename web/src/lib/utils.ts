import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

const KiB = 1024;
const MiB = KiB * 1024;
const GiB = MiB * 1024;

export function humanSize(b: number): string {
	if (b >= GiB) return `${(b / GiB).toFixed(2)} GiB`;
	if (b >= MiB) return `${(b / MiB).toFixed(2)} MiB`;
	if (b >= KiB) return `${(b / KiB).toFixed(1)} KiB`;
	return `${b} B`;
}

export function shortTime(rfc3339: string | undefined | null): string {
	if (!rfc3339) return '';
	const d = new Date(rfc3339);
	if (isNaN(d.getTime())) return rfc3339;
	return d.toLocaleString(undefined, {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	});
}

export function relativeTime(rfc3339: string | undefined | null): string {
	if (!rfc3339) return '';
	const d = new Date(rfc3339);
	if (isNaN(d.getTime())) return rfc3339;
	const diffMs = d.getTime() - Date.now();
	const abs = Math.abs(diffMs);
	const minutes = Math.round(abs / 60_000);
	const hours = Math.round(abs / 3_600_000);
	const days = Math.round(abs / 86_400_000);

	const past = diffMs < 0;
	const unit = (n: number, u: string) => `${n} ${u}${n === 1 ? '' : 's'}`;

	if (days >= 1) return past ? `${unit(days, 'day')} ago` : `in ${unit(days, 'day')}`;
	if (hours >= 1) return past ? `${unit(hours, 'hour')} ago` : `in ${unit(hours, 'hour')}`;
	if (minutes >= 1) return past ? `${unit(minutes, 'min')} ago` : `in ${unit(minutes, 'min')}`;
	return past ? 'just now' : 'soon';
}

export function usesLabel(n: number): string {
	return n === 1 ? 'one download' : `${n} downloads`;
}
