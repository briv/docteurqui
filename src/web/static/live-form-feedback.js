import { makeElement } from "./utils";

export const Validators = {
    Required: (field) => field.trim().length > 0,
    Length: (length) => (field) => field.length === length,
    Number: (min, max) => (field) => {
        const num = Number.parseFloat(field);
        const isNumber = Number.isNaN(num) === false;
        return isNumber && (num >= min) && (num <= max);
    },
};

const ErrorClass = 'error-feedback';
const InputInvalidClass = 'invalid';

const clearErrorMessage = (parent, input) => {
    const errEl = parent.querySelector(`.${ErrorClass}`);
    if (errEl) {
        errEl.remove();
    }
    input.classList.remove(InputInvalidClass);
}

const setErrorMessage = (parent, input, message) => {
    const errEl = parent.querySelector(`.${ErrorClass}`);

    if (errEl) {
        const messageEl = errEl.querySelector('div');
        messageEl.textContent = message;
    } else {
        const newErrEl = makeElement('div', el => {
            el.classList.add(ErrorClass);

            const messageEl = makeElement('div', el => {
                el.textContent = message;
            });
            el.appendChild(messageEl);
        });
        parent.appendChild(newErrEl);
    }
    input.classList.add(InputInvalidClass);
};

const errorCheck = (parent, inputEl, isValid, errorMessage) => {
    if (isValid === false) {
        setErrorMessage(parent, inputEl, errorMessage);
    } else {
        clearErrorMessage(parent, inputEl);
    }
};

// Before first blur, no error message is shown.
// After first blur, feedback is updated after each change.
// NOTE: for an "input" element, we can use input.labels to get the labels.
export const setupFieldFeedback = (form, formFeedbackParentQuerySelector, fieldFeedbackSetup) => {
    // TODO: just for now
    if (!fieldFeedbackSetup.check) {
        return;
    }

    const inputEl = form.querySelector(fieldFeedbackSetup.querySelector);
    const parent = inputEl.closest(formFeedbackParentQuerySelector);

    let hasBeenBlurredOnce = false;

    inputEl.addEventListener('input', (e) => {
        if (hasBeenBlurredOnce === false) {
            return;
        }

        const currentValue = e.currentTarget.value;
        const isValid = fieldFeedbackSetup.check(currentValue);
        errorCheck(parent, inputEl, isValid, fieldFeedbackSetup.message);
    });

    inputEl.addEventListener('blur', (e) => {
        if (hasBeenBlurredOnce === false) {
            hasBeenBlurredOnce = true;
        } else {
            return;
        }

        const currentValue = e.currentTarget.value;
        const isValid = fieldFeedbackSetup.check(currentValue);
        errorCheck(parent, inputEl, isValid, fieldFeedbackSetup.message);
    });
};
