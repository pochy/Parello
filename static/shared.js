window.GolangKanban = (() => {
	function csrfToken() {
		return document.querySelector('meta[name="csrf-token"]')?.content
			|| document.querySelector('input[name="_csrf"]')?.value
			|| '';
	}

	async function patchJSON(url, payload) {
		const response = await fetch(url, {
			method: 'PATCH',
			credentials: 'same-origin',
			headers: {
				'Content-Type': 'application/json',
				'Accept': 'application/json',
				'X-CSRF-Token': csrfToken(),
			},
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
