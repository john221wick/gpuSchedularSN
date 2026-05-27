export const commandPathKey = 'gpusched.commandPath';

export function normalizeCommandPath(value) {
	return String(value || '').trim();
}

export function getCommandPath() {
	if (typeof localStorage === 'undefined') {
		return '';
	}
	return normalizeCommandPath(localStorage.getItem(commandPathKey));
}

export function saveCommandPath(value) {
	if (typeof localStorage === 'undefined') {
		return '';
	}

	const path = normalizeCommandPath(value);
	if (path) {
		localStorage.setItem(commandPathKey, path);
	} else {
		localStorage.removeItem(commandPathKey);
	}
	return path;
}

export function clearCommandPath() {
	if (typeof localStorage !== 'undefined') {
		localStorage.removeItem(commandPathKey);
	}
	return '';
}
