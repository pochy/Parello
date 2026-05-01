document.addEventListener('alpine:init', () => {
	const shared = window.GolangKanban;

	Alpine.data('kanbanBoard', (boardId) => ({
		boardId,
		sortError: '',
		cardError: '',
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
			shared.initTree(document.querySelector(selector));
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
				shared.initTree(dialog.querySelector(`[data-card-fragment="${fragment}"]`));
			}
		},
		async submitCardForm(event) {
			const form = event.target;
			const dialog = form.closest('dialog[id^="card-"]');
			const cardId = shared.cardIDFromDialog(dialog);
			if (!dialog || !cardId) {
				return;
			}
			this.cardError = '';
			form.setAttribute('aria-busy', 'true');
			try {
				const result = await shared.fetchFormHTML(form, event.submitter);
				if (result.errorCode) {
					this.cardError = shared.cardFormErrorMessage(result.errorCode);
					return;
				}
				const nextDialog = result.doc.getElementById(dialog.id);
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
				this.replaceCardTile(cardId, result.doc);
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
			try {
				await shared.patchJSON(url, payload);
			} catch (error) {
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
