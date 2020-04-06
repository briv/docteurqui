import { makeElement } from "./utils";
import { fillFormField } from "./form-fill";

const AutoCompleteURLPath = '/b/search-doctor';
const MinQueryLength = 3;

const shouldAutoCompleteForValue = (value, previousValue) => {
    if (value.length < MinQueryLength) {
        return false;
    }
    if (value !== previousValue) {
        return true;
    }
    return false;
};

const onUserSelectAutoCompleteItem = (result, input, formPart) => () => {
    const form = input.form;
    console.log("selected", result);
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

        el.appendChild(ul);
    });

    parentNode.appendChild(list);
};

export const makeInputAutocomplete = (input) => {
    const fieldset = input.closest('fieldset');
    if (!fieldset) {
        return;
    }
    const formPart = fieldset.dataset.enhancedFormPart;

    let previousValue = null;
    input.addEventListener('blur', (e) => {
        // Do not remove our autocomplete list on blur if the user just selected one of the items:
        // the 'click' handler won't fire otherwise !
        if (e.relatedTarget && e.relatedTarget.dataset.isAutoCompleteItem) {
            return;
        }
        const inputEl = e.currentTarget;
        removeAutoCompleteList(inputEl);
    });

    input.addEventListener('input', (e) => {
        const inputEl = e.currentTarget;
        const { value } = inputEl;

        const parentNode = inputEl.closest('.autocomplete-parent');

        if (shouldAutoCompleteForValue(value, previousValue) === false) {
            return;
        }
        previousValue = value;

        const url = new URL(AutoCompleteURLPath, document.URL);
        const params = new URLSearchParams();
        params.set('query', value);
        url.search = params.toString();

        const query = fetch(url, {
            method: 'GET',
            cache: 'no-cache',
            credentials: 'omit',
            redirect: 'follow',
        });

        query.then(async (response) => {
            if (response.ok) {
                try {
                    const results = await response.json();
                    createAutoCompleteList(parentNode, inputEl, formPart, results.matches);
                } catch (err) {

                }
            }
        });

        query.catch(() => {
            // TODO: do nothing if it fails ?
        });
    });
};
