window.GolangKanban = (() => {
	function cardFormErrorMessage(code) {
		switch (code) {
			case 'card_title_required':
				return 'カード名を入力してください。';
			case 'checklist_required':
				return 'チェックリスト名を入力してください。';
			case 'comment_required':
				return 'コメントを入力してください。';
			case 'attachment_required':
				return '添付リンクの名前と URL を入力してください。';
			case 'due_invalid':
			case 'date_invalid':
				return '開始日と期限の日付を確認してください。';
			default:
				return '保存できませんでした。入力内容を確認してください。';
		}
	}

	function cardIDFromDialog(dialog) {
		const cardId = (dialog?.id || '').replace('card-', '');
		return /^\d+$/.test(cardId) ? cardId : '';
	}

	function formBody(form, submitter) {
		const data = new URLSearchParams();
		for (const [key, value] of new FormData(form)) {
			data.append(key, value);
		}
		if (submitter?.name && !data.has(submitter.name)) {
			data.append(submitter.name, submitter.value);
		}
		if (!data.has('return_to')) {
			data.append('return_to', `${window.location.pathname}${window.location.search}`);
		}
		return data;
	}

	function replaceElement(current, next) {
		if (!current || !next) {
			return null;
		}
		Alpine.mutateDom(() => {
			current.outerHTML = next.outerHTML;
		});
		return current;
	}

	function initTree(node) {
		if (node) {
			Alpine.initTree(node);
		}
	}

	async function fetchFormHTML(form, submitter) {
		const response = await fetch(form.action, {
			method: (form.method || 'POST').toUpperCase(),
			headers: {
				'Content-Type': 'application/x-www-form-urlencoded',
				'X-Requested-With': 'fetch',
			},
			body: formBody(form, submitter),
		});
		const responseURL = new URL(response.url || window.location.href, window.location.href);
		const errorCode = responseURL.searchParams.get('error');
		if (!response.ok || errorCode) {
			return { response, errorCode, doc: null };
		}
		const html = await response.text();
		return {
			response,
			errorCode: '',
			doc: new DOMParser().parseFromString(html, 'text/html'),
		};
	}

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
		cardFormErrorMessage,
		cardIDFromDialog,
		fetchFormHTML,
		initTree,
		patchJSON,
		replaceElement,
	};
})();
