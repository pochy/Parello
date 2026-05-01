document.addEventListener('alpine:init', () => {
	const shared = window.GolangKanban;

	function clamp(value, min, max) {
		return Math.min(Math.max(value, min), max);
	}

	function parseLocalDate(value) {
		const [year, month, day] = value.split('-').map(Number);
		return new Date(year, month - 1, day);
	}

	function formatLocalDate(date) {
		const year = date.getFullYear();
		const month = String(date.getMonth() + 1).padStart(2, '0');
		const day = String(date.getDate()).padStart(2, '0');
		return `${year}-${month}-${day}`;
	}

	function addDays(value, days) {
		const date = parseLocalDate(value);
		date.setDate(date.getDate() + days);
		return formatLocalDate(date);
	}

	Alpine.data('timelineBoard', (boardId, fromDate, dayCount, cellWidth) => ({
		boardId,
		fromDate,
		dayCount,
		cellWidth,
		timelineError: '',
		cardError: '',
		activeDrag: null,
		suppressClick: false,
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
		replaceTimelineCard(cardId, doc) {
			const current = document.querySelector(`[data-timeline-card][data-card-id="${cardId}"]`);
			const next = doc.querySelector(`[data-timeline-card][data-card-id="${cardId}"]`);
			if (!current || !next) {
				return;
			}
			Alpine.mutateDom(() => {
				current.outerHTML = next.outerHTML;
			});
			shared.initTree(document.querySelector(`[data-timeline-card][data-card-id="${cardId}"]`));
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
				this.replaceTimelineCard(cardId, result.doc);
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
		openCard(event, dialogId) {
			if (this.suppressClick) {
				event.preventDefault();
				this.suppressClick = false;
				return;
			}
			document.getElementById(dialogId)?.showModal();
		},
		startTimelineDrag(event) {
			if (event.button !== 0) {
				return;
			}
			const card = event.currentTarget;
			const handle = event.target.closest('[data-timeline-handle]')?.dataset.timelineHandle || 'move';
			const startOffset = Number(card.dataset.startOffset);
			const dueOffset = Number(card.dataset.dueOffset);
			const move = (moveEvent) => this.updateTimelineDrag(moveEvent);
			const up = (upEvent) => this.finishTimelineDrag(upEvent);
			this.timelineError = '';
			this.activeDrag = {
				card,
				cardId: card.dataset.cardId,
				handle,
				startX: event.clientX,
				startOffset,
				dueOffset,
				deltaDays: 0,
				didMove: false,
				originalLeft: card.style.left,
				originalTop: card.style.top,
				originalWidth: card.style.width,
				move,
				up,
			};
			document.body.classList.add('timeline-dragging');
			card.setPointerCapture(event.pointerId);
			window.addEventListener('pointermove', move);
			window.addEventListener('pointerup', up, { once: true });
		},
		updateTimelineDrag(event) {
			if (!this.activeDrag) {
				return;
			}
			const drag = this.activeDrag;
			const deltaPixels = event.clientX - drag.startX;
			const deltaDays = Math.round(deltaPixels / this.cellWidth);
			if (Math.abs(deltaPixels) > 3) {
				drag.didMove = true;
			}
			drag.deltaDays = deltaDays;
			const next = this.nextTimelineRange(drag);
			this.applyTimelineCardPosition(drag.card, next.startIndex, next.dueIndex);
		},
		async finishTimelineDrag() {
			if (!this.activeDrag) {
				return;
			}
			const drag = this.activeDrag;
			window.removeEventListener('pointermove', drag.move);
			document.body.classList.remove('timeline-dragging');
			this.activeDrag = null;
			if (!drag.didMove || drag.deltaDays === 0) {
				this.restoreTimelineCard(drag);
				return;
			}
			this.suppressClick = true;
			window.setTimeout(() => {
				this.suppressClick = false;
			}, 0);
			const next = this.nextTimelineRange(drag);
			const startAt = addDays(this.fromDate, next.startIndex);
			const dueAt = addDays(this.fromDate, next.dueIndex);
			try {
				const response = await shared.patchJSON(`/api/cards/${drag.cardId}/timeline`, { startAt, dueAt });
				drag.card.dataset.start = response.startAt;
				drag.card.dataset.due = response.dueAt;
				drag.card.dataset.startOffset = String(next.startIndex);
				drag.card.dataset.dueOffset = String(next.dueIndex);
				drag.card.title = `${drag.card.dataset.title || ''} ${response.startAt} - ${response.dueAt}`.trim();
			} catch (error) {
				this.restoreTimelineCard(drag);
				this.timelineError = 'タイムラインの日付を保存できませんでした。ページを再読み込みしてください。';
			}
		},
		nextTimelineRange(drag) {
			const duration = Math.max(1, drag.dueOffset - drag.startOffset + 1);
			if (drag.handle === 'start') {
				const maxStart = Math.min(drag.dueOffset, this.dayCount - 1);
				const startIndex = clamp(drag.startOffset + drag.deltaDays, 0, maxStart);
				return { startIndex, dueIndex: Math.max(startIndex, drag.dueOffset) };
			}
			if (drag.handle === 'end') {
				const minDue = Math.max(0, drag.startOffset);
				const dueIndex = clamp(drag.dueOffset + drag.deltaDays, minDue, this.dayCount - 1);
				return { startIndex: Math.min(drag.startOffset, dueIndex), dueIndex };
			}
			const maxStart = Math.max(0, this.dayCount - duration);
			const startIndex = clamp(drag.startOffset + drag.deltaDays, 0, maxStart);
			return { startIndex, dueIndex: startIndex + duration - 1 };
		},
		applyTimelineCardPosition(card, startIndex, dueIndex) {
			const visibleStart = clamp(startIndex, 0, this.dayCount - 1);
			const visibleDue = clamp(dueIndex, 0, this.dayCount - 1);
			const left = visibleStart * this.cellWidth + 8;
			const width = Math.max(56, (visibleDue - visibleStart + 1) * this.cellWidth - 16);
			card.style.left = `${left}px`;
			card.style.width = `${width}px`;
		},
		restoreTimelineCard(drag) {
			drag.card.style.left = drag.originalLeft;
			drag.card.style.top = drag.originalTop;
			drag.card.style.width = drag.originalWidth;
		},
	}));
});
