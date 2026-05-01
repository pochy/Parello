document.addEventListener('alpine:init', () => {
	Alpine.data('kanbanBoard', (boardId) => ({
		boardId,
		sortError: '',
		cardError: '',
		cardIDFromDialog(dialog) {
			const cardId = (dialog?.id || '').replace('card-', '');
			return /^\d+$/.test(cardId) ? cardId : '';
		},
		cardFormErrorMessage(code) {
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
					return '期限の日付を確認してください。';
				default:
					return '保存できませんでした。入力内容を確認してください。';
			}
		},
		replaceCardTile(cardId, doc) {
			if (!cardId) {
				return;
			}
			const selector = `[data-card-id="${cardId}"]`;
			const currentTile = document.querySelector(selector);
			const nextTile = doc.querySelector(selector);
			if (!currentTile || !nextTile) {
				return;
			}
			Alpine.mutateDom(() => {
				currentTile.outerHTML = nextTile.outerHTML;
			});
			const updatedTile = document.querySelector(selector);
			if (updatedTile) {
				Alpine.initTree(updatedTile);
			}
		},
		replaceCardFragments(dialog, nextDialog, fragments) {
			for (const fragment of fragments) {
				const current = dialog.querySelector(`[data-card-fragment="${fragment}"]`);
				const next = nextDialog.querySelector(`[data-card-fragment="${fragment}"]`);
				if (!current || !next) {
					continue;
				}
				Alpine.mutateDom(() => {
					current.outerHTML = next.outerHTML;
				});
				const updated = dialog.querySelector(`[data-card-fragment="${fragment}"]`);
				if (updated) {
					Alpine.initTree(updated);
				}
			}
		},
		async submitCardForm(event) {
			const form = event.target;
			const dialog = form.closest('dialog[id^="card-"]');
			const cardId = this.cardIDFromDialog(dialog);
			if (!dialog || !cardId) {
				return;
			}
			const data = new URLSearchParams();
			for (const [key, value] of new FormData(form)) {
				data.append(key, value);
			}
			const submitter = event.submitter;
			if (submitter?.name && !data.has(submitter.name)) {
				data.append(submitter.name, submitter.value);
			}
			this.cardError = '';
			form.setAttribute('aria-busy', 'true');
			try {
				const response = await fetch(form.action, {
					method: (form.method || 'POST').toUpperCase(),
					headers: {
						'Content-Type': 'application/x-www-form-urlencoded',
						'X-Requested-With': 'fetch',
					},
					body: data,
				});
				const responseURL = new URL(response.url || window.location.href, window.location.href);
				const errorCode = responseURL.searchParams.get('error');
				if (!response.ok || errorCode) {
					this.cardError = this.cardFormErrorMessage(errorCode);
					return;
				}
				const html = await response.text();
				const doc = new DOMParser().parseFromString(html, 'text/html');
				const nextDialog = doc.getElementById(dialog.id);
				if (!nextDialog) {
					this.cardError = '更新後のカードを読み込めませんでした。ページを再読み込みしてください。';
					return;
				}
				const scroller = dialog.querySelector('article');
				const scrollTop = scroller ? scroller.scrollTop : 0;
				const fragments = (form.dataset.refreshFragments || form.closest('[data-card-fragment]')?.dataset.cardFragment || '')
					.split(/\s+/)
					.filter(Boolean);
				this.replaceCardFragments(dialog, nextDialog, [...new Set(fragments)]);
				this.replaceCardTile(cardId, doc);
				this.$nextTick(() => {
					const updatedScroller = dialog.querySelector('article');
					if (updatedScroller) {
						updatedScroller.scrollTop = Math.min(scrollTop, updatedScroller.scrollHeight);
					}
				});
			} catch (error) {
				this.cardError = '通信に失敗しました。ネットワークとサーバーを確認してください。';
			} finally {
				form.removeAttribute('aria-busy');
			}
		},
		async patchJSON(url, payload) {
			this.sortError = '';
			const response = await fetch(url, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload),
			});
			if (!response.ok) {
				this.sortError = '並び順を保存できませんでした。ページを再読み込みしてください。';
			}
		},
		reorderLists(el) {
			const listIds = [...el.querySelectorAll(':scope > [data-list-id]')].map((node) => Number(node.dataset.listId));
			this.patchJSON('/api/lists/reorder', { boardId: this.boardId, listIds });
		},
		reorderCards(el) {
			const toListId = Number(el.dataset.listId);
			const cardIds = [...el.querySelectorAll(':scope > [data-card-id]')].map((node) => Number(node.dataset.cardId));
			this.patchJSON('/api/cards/reorder', { toListId, cardIds });
		},
	}));
});
