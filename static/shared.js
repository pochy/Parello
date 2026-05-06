window.GolangKanban = (() => {
	async function patchJSON(url, payload) {
		const response = await fetch(url, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(payload),
		});
		if (!response.ok) {
			throw new Error(`PATCH ${url} failed with ${response.status}`);
		}
		if (response.status === 204) {
			return null;
		}
		return response.json();
	}

	return {
		patchJSON,
	};
})();
