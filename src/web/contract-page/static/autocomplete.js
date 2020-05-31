import { makeElement } from "./utils";
import { fillFormField } from "./form-fill";

const AutoCompleteURLPath = 'b/search-doctor';
const MinQueryLength = 3;

const MsDurationToHideOldResultsIfWaitIsTooLong = 1000;

const CustomEventPrefix = 'com.docteurqui.autocompletelist';

const shouldAutoCompleteForValue = (value, previousRequestInfo) => {
    if (value.length < MinQueryLength) {
        return false;
    }
    if (!previousRequestInfo.value || value !== previousRequestInfo.value) {
        return true;
    }
    return false;
};

const onUserSelectAutoCompleteItem = (result, input, formPart) => () => {
    const form = input.form;
    Object.entries(result).forEach(([key, value]) => {
        fillFormField(form, `${formPart}-${key}`, value);
    });
    removeAutoCompleteList(input);
};

const removeAutoCompleteList = (input) => {
    const parentNode = input.closest('.autocomplete-parent');
    const list = parentNode.querySelector('.autocomplete-list');
    if (list) {
        list.remove();
    }
};

const createAutoCompleteList = (parentNode, input, formPart, results) => {
    const getListEl = (f) => {
        const alreadyListEl = parentNode.querySelector('.autocomplete-list');
        if (alreadyListEl) {
            while (alreadyListEl.firstChild) {
                alreadyListEl.removeChild(alreadyListEl.lastChild);
            }
            f(alreadyListEl);
            return alreadyListEl;
        }

        return makeElement('div', el => {
            el.classList.add('autocomplete-list');
            f(el);
        })
    };

    const list = getListEl(el => {
        const ul = makeElement('ul', el => {
            results.forEach((resultItem) => {
                const itemEl = makeElement('li');
                itemEl.setAttribute('tabindex', '0');
                itemEl.dataset.isAutoCompleteItem = 'true';
                itemEl.textContent = resultItem.name;

                itemEl.addEventListener('click', onUserSelectAutoCompleteItem(resultItem, input, formPart));
                el.appendChild(itemEl);
            });
        });

        // Allow the user to use "enter" to select item and arrow keys to cycle between autocomplete items.
        ul.addEventListener('keydown', handleKeyShortcuts);

        // Allow the user to "tab-out" of the autocomplete list.
        ul.addEventListener('focusout', e => {
            // To do this, we observe whether the focus has been moved to a target which is *NOT* the input or an autocomplete item.
            const isAutoCompleteItem = e.relatedTarget && e.relatedTarget.dataset.isAutoCompleteItem;
            const focusIsRelatedToAutocomplete = isAutoCompleteItem || e.relatedTarget === input;
            if (focusIsRelatedToAutocomplete === false) {
                removeAutoCompleteList(input);
            }
        });

        el.appendChild(ul);
    });

    parentNode.appendChild(list);
};

const handleKeyShortcuts = (e) => {
    const { key } = e;
    if (key === 'ArrowDown' || key === 'ArrowUp' || key === 'Enter') {
        const customEvent = new Event(`${CustomEventPrefix}.${key}`,
        {
            bubbles: true,
            cancelable: true,
        });
        const handled = e.currentTarget.dispatchEvent(customEvent);
        if (handled === true) {
            e.preventDefault();
        }
    }
};

const listenKeyShortCuts = (input) => (e) => {
    // Check if autocomplete list is displayed.
    const root = e.currentTarget;
    const list = root.querySelector('.autocomplete-list');
    if (!list) {
        // If list is not displayed, cancel event to use default actions for keys.
        e.preventDefault();
        return;
    }

    // At this point, we know the list is displayed.
    const ul = list.firstChild;

    // Handle the Enter key in a specific way.
    if (e.type === `${CustomEventPrefix}.Enter`) {
        if (e.target === input) {
            // Don't handle the case when the input is focused.
            e.preventDefault();
        } else if (ul.contains(document.activeElement)) {
            document.activeElement.click();
        }
        return;
    }

    const elementToFocus = (() => {
        if (document.activeElement === input) {
            if (e.type === `${CustomEventPrefix}.ArrowDown`) {
                return ul.firstChild;
            }
        } else if (ul.contains(document.activeElement)) {
            if (e.type === `${CustomEventPrefix}.ArrowDown`) {
                return document.activeElement.nextElementSibling;
            } else if (e.type === `${CustomEventPrefix}.ArrowUp`) {
                return document.activeElement.previousElementSibling || input;
            }
        }
        return null;
    })();

    if (!elementToFocus) {
        if (e.type !== `${CustomEventPrefix}.ArrowDown`) {
            e.preventDefault();
        }
        return;
    }

    elementToFocus.focus();
};

export const InputAutocompleter = function (input) {
    const fieldset = input.closest('fieldset');
    if (!fieldset) {
        throw new Error('InputAutocompleter expects an ancestor <fieldset> element');
    }
    const formPart = fieldset.dataset.enhancedFormPart;

    let previousRequestInfo = {};
    const resetPreviousRequestInfo = () => { previousRequestInfo = {}; };

    let isCurrentlyFocused = false;
    const handleBlur = (e) => {
        isCurrentlyFocused = false;
        resetPreviousRequestInfo();

        // Do not remove our autocomplete list on blur if the user just selected one of the items:
        // the 'click' handler won't fire otherwise !
        if (e.relatedTarget && e.relatedTarget.dataset.isAutoCompleteItem) {
            return;
        }
        const inputEl = e.currentTarget;
        removeAutoCompleteList(inputEl);
    };

    const handleInput = (e) => {
        isCurrentlyFocused = true;
        if (previousRequestInfo.abortController) {
            previousRequestInfo.abortController.abort();
        }

        const inputEl = e.currentTarget;
        const { value } = inputEl;

        if (shouldAutoCompleteForValue(value, previousRequestInfo) === false) {
            resetPreviousRequestInfo();
            removeAutoCompleteList(inputEl);
            return;
        }

        const url = new URL(AutoCompleteURLPath, document.URL);
        const params = new URLSearchParams();
        params.set('query', value);
        url.search = params.toString();

        const abortController = new AbortController();

        previousRequestInfo = {
            abortController,
            value,
        };

        // If the new request is taking too long, remove old autocomplete results.
        const hideOldResultsIfWaitIsTooLongTimerId = setTimeout(() => {
            removeAutoCompleteList(inputEl);
        }, MsDurationToHideOldResultsIfWaitIsTooLong);

        // If a request is aborted, we need to clear the timer otherwise it will fire for sure.
        abortController.signal.addEventListener('abort', () => {
            clearTimeout(hideOldResultsIfWaitIsTooLongTimerId);
        });

        fetch(url, {
            method: 'GET',
            cache: 'no-cache',
            credentials: 'omit',
            redirect: 'follow',
            signal: abortController.signal,
        }).then(async (response) => {
            clearTimeout(hideOldResultsIfWaitIsTooLongTimerId);
            if (response.ok) {
                try {
                    const results = await response.json();
                    const parentNode = inputEl.closest('.autocomplete-parent');
                    // Since the response is asynchronous, we might end up here after the user has
                    // changed focus to a different input field. We don't want autocomplete results
                    // popping up in that case.
                    if (isCurrentlyFocused) {
                        createAutoCompleteList(parentNode, inputEl, formPart, results.matches);
                    }
                } catch (err) {
                }
            } else {
                // In case of a non 2xx status, hide the autocomplete list if it is there.
                removeAutoCompleteList(inputEl);
            }
        }).catch((err) => {
            // Do nothing in case a query fails (even if due to an abort()).
        });
    };

    // This boolean allows setup() and remove() calls to be used without fear
    // of calling one more times than the other.
    let isSetup = false;

    const listenKeyShortCutsForInput = listenKeyShortCuts(input);
    this.setup = () => {
        if (isSetup === false) {
            isSetup = true;
            input.addEventListener('blur', handleBlur);
            input.addEventListener('input', handleInput);
            input.addEventListener('keydown', handleKeyShortcuts);

            const parentNode = input.closest('.autocomplete-parent');
            ['ArrowUp', 'ArrowDown', 'Enter'].forEach(key => {
                parentNode.addEventListener(`${CustomEventPrefix}.${key}`, listenKeyShortCutsForInput);
            });
        }
    };

    this.remove = () => {
        if (isSetup === true) {
            isSetup = false;
            input.removeEventListener('blur', handleBlur);
            input.removeEventListener('input', handleInput);
            input.removeEventListener('keydown', handleKeyShortcuts);

            const parentNode = input.closest('.autocomplete-parent');
            ['ArrowUp', 'ArrowDown', 'Enter'].forEach(key => {
                parentNode.removeEventListener(`${CustomEventPrefix}.${key}`, listenKeyShortCutsForInput);
            });
        }
    };
};
