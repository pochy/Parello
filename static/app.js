document.addEventListener('alpine:init', () => {
	const shared = window.GolangKanban;

	Alpine.data('kanbanBoard', (boardId) => ({
		boardId,
		sortError: '',
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
