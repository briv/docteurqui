import { makeElement, smoothScrollTo } from "./utils";
import { FormValidationError } from "./errors";

export const FormValidationIssues = {
    MissingRequired: 'input can not be empty',
	TooMany: 'too many values',
	ParseError: 'could not parse input',
	LengthError: 'unexpected input length',
};

export const Validators = {
    Required: (field) => {
        if (field.trim().length > 0) {
            return true;
        }
        return FormValidationIssues.MissingRequired;
    },
    Length: (length) => (field) => {
        if (field.length === length) {
            return true;
        }
        return FormValidationIssues.LengthError;
    },
    Number: (min, max) => (field) => {
        const num = Number.parseFloat(field);
        const isNumber = Number.isNaN(num) === false;
        const isValid = isNumber && (num >= min) && (num <= max);
        if (isValid) {
            return true;
        }
        return FormValidationIssues.ParseError;
    },
};


const FormFeedbackParentQuerySelector = '.single-form-input-group';

const ErrorClass = 'error-feedback';
const InputInvalidClass = 'invalid';

const getFormErrorRecapContainer = (form) => form.querySelector('.call-to-action.error');

const createContentForError = (errorHandler, root, error) => {
    if (! (error instanceof FormValidationError)) {
        root.textContent = error.message;
        return;
    }

    // Special handling for period-start and period-end. If both validation errors are present,
    // only keep one.
    if (error.issues.hasOwnProperty('period-start') && error.issues.hasOwnProperty('period-end')) {
        delete error.issues['period-end'];
    }

    const errorDescription = makeElement('div', el => {
        el.textContent = 'Zut, il y a quelques soucis à régler dans le formulaire:';
    });
    root.appendChild(errorDescription);

    const ul = makeElement('ul');

    const sortedIssues = Object.entries(error.issues).sort((a, b) => {
        const aIdx = errorHandler.feedbackDescriptions.findIndex(el => el.name === a[0]);
        const bIdx = errorHandler.feedbackDescriptions.findIndex(el => el.name === b[0]);
        return aIdx - bIdx;
    });
    for (const [issueName, value] of sortedIssues) {
        const feedBackDescription = getCorrespondingFeedbackDescription(errorHandler, issueName);
        if (!feedBackDescription) {
            continue;
        }

        compensateScrollDuring(() => {
            errorHandler.notifyError(feedBackDescription, value);
        });

        const allInputs = errorHandler.form.querySelectorAll(feedBackDescription.querySelector);
        const firstInput = allInputs[0];

        const newErrItem = makeElement('li', el => {
            el.dataset['errorId'] = issueName;

            const a = makeElement('a', el => {
                el.textContent = feedBackDescription.errorLink;
                el.setAttribute('href', '');

                let shouldFocus = true;
                // Special handling for period inputs.
                const { name } = feedBackDescription;
                if (name === 'period-start' || name === 'period-end') {
                    shouldFocus = false;
                }

                el.addEventListener('click', e => {
                    e.preventDefault();

                    firstInput.labels[0].scrollIntoView();
                    if (shouldFocus) {
                        firstInput.focus();
                    }
                });
            });
            el.appendChild(a);
        });
        ul.appendChild(newErrItem);
    }
    root.appendChild(ul);
};

const getCorrespondingFeedbackDescription = (errorHandler, name) => {
    return errorHandler.feedbackDescriptions.find(el => el.name === name);
};

// This function tries to keep the form submit button at the same top-relative position compared to the current scroll position.
const compensateScrollDuring = (f) => {
    const submitButton = document.querySelector('button[type="submit"]');

    const topOffsetOfButtonRelativeToScroll = () => {
        return submitButton.offsetTop - document.scrollingElement.scrollTop;
    };

    const before = topOffsetOfButtonRelativeToScroll();
    f();
    const diff = topOffsetOfButtonRelativeToScroll() - before;

    if (diff !== 0) {
        document.scrollingElement.scrollTop += diff;
    }
};

// Clears all errors from the recap
const clearFormErrorRecap = (errorHandler) => {
    const errorContainer = getFormErrorRecapContainer(errorHandler.form);
    if (!errorContainer) {
        return;
    }

    // Reset all error elements within form.
    errorHandler.feedbackDescriptions.forEach(f => {
        compensateScrollDuring(() => {
            errorHandler.clearError(f);
        });
    });

    // Just in case, but this should never happen.
    errorContainer.remove();
};

export function FormErrorHandler(form, formFeedbackDescriptions) {
    const eh = (error) => {
        let errorContainer = getFormErrorRecapContainer(form);
        if (!errorContainer) {
            errorContainer = makeElement('div', el => {
                el.classList.add('call-to-action', 'error');
            });
            form.appendChild(errorContainer);
        }

        errorContainer.appendChild(makeElement('div', el => {
            createContentForError(eh, el, error);
            el.classList.add('error-animation', 'd-flex', 'flex-column');
        }));

        // Only scroll for the user if they can't already see that there is a list of errors.
        const rect = errorContainer.getBoundingClientRect();
        const verticalPixelsToShowAtLeast = 70;
        const isVisibleEnough =
            rect.top < (document.scrollingElement.clientHeight - verticalPixelsToShowAtLeast)
            && rect.bottom >= 0;
        if (isVisibleEnough === false) {
            setTimeout(() => { smoothScrollTo(errorContainer); }, 100);
        }
    };

    eh.form = form;
    eh.feedbackDescriptions = formFeedbackDescriptions;

    eh.notifyError = (feedbackDescription, error) => {
        const inputEl = form.querySelector(feedbackDescription.querySelector);
        const parent = getInputParentForErrorElement(inputEl);

        const message = (() => {
            const { overridingMesssage } = feedbackDescription;
            if (overridingMesssage) {
                if (typeof overridingMesssage === 'function') {
                    return overridingMesssage(error);
                }
                return overridingMesssage;
            }
            if (error === FormValidationIssues.MissingRequired) {
                return 'Ce champ doit être renseigné.';
            }
            if (error === FormValidationIssues.ParseError) {
                return "Ce champ a une valeur inattendue.";
            }
            if (error === FormValidationIssues.LengthError) {
                return "Ce champ a une valeur inattendue.";
            }
            if (error === FormValidationIssues.TooMany) {
                return "Le nombre de valeurs de ce champ a dépassé une limite.";
            }

            return error;
        })();

        setErrorMessage(parent, inputEl, message);
    };

    eh.clearError = (feedbackDescription) => {
        const inputEl = form.querySelector(feedbackDescription.querySelector);
        const parent = getInputParentForErrorElement(inputEl);
        clearErrorMessage(parent, inputEl);

        // Also clear the error from the "global form errors" area if needed.
        const errContainer = getFormErrorRecapContainer(form);
        if (!errContainer) {
            return;
        }
        const globalErrEl = errContainer.querySelector(`[data-error-id="${feedbackDescription.name}"]`);
        if (globalErrEl) {
            globalErrEl.remove();
            if (errContainer.firstChild.lastChild.childElementCount === 0) {
                errContainer.remove();
            }
        }
    };

    eh.checkField = function (feedbackDescription, currentValue) {
        const validityCheck = feedbackDescription.check(currentValue);

        if (validityCheck === true) {
            this.clearError(feedbackDescription);
        } else {
            this.notifyError(feedbackDescription, validityCheck);
        }
    };

    // For these fields with no specific client-side validation, the error message will be
    // cleaned up by the global form handling.
    const uncheckedFieldListener = (e) => {
        const target = e.target;
        if (target.tagName !== 'INPUT') {
            return;
        }
        const name = target.getAttribute('name');
        const feedBackDescription = getCorrespondingFeedbackDescription(eh, name);
        if (!feedBackDescription) {
            return;
        }

        // Special case for periods.
        if (name === 'period-start' || name === 'period-end') {
            eh.clearError(getCorrespondingFeedbackDescription(eh, 'period-start'));
            eh.clearError(getCorrespondingFeedbackDescription(eh, 'period-end'));
            return;
        }

        if (feedBackDescription.check) {
            eh.checkField(feedBackDescription, target.value);
        } else {
            eh.clearError(feedBackDescription);
        }
    };
    form.addEventListener('change', uncheckedFieldListener);

    formFeedbackDescriptions.forEach(el => { setupFieldFeedback(eh, el) });

    this.handle = (err) => { eh(err); };
    this.clear = () => { clearFormErrorRecap(eh); };
};

const clearErrorMessage = (parent, input) => {
    const errEl = parent.querySelector(`.${ErrorClass}`);
    if (errEl) {
        errEl.remove();
    }
    if (input) {
        input.classList.remove(InputInvalidClass);
    }
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

const getInputParentForErrorElement = (inputEl) => {
    const parent = inputEl.closest(FormFeedbackParentQuerySelector);
    return parent;
};

// Before first blur, no error message is shown.
// After first blur, feedback is updated after each change.
const setupFieldFeedback = (errorHandler, feedBackDescription) => {
    // Only setup live validation feedback if a 'check' property is set.
    if (!feedBackDescription.check) {
        return;
    }

    const inputs = errorHandler.form.querySelectorAll(feedBackDescription.querySelector);
    let shouldDisplayErrorFeedback = false;
    const inputEl = inputs[0];

    inputEl.addEventListener('input', (e) => {
        if (shouldDisplayErrorFeedback === false) {
            const noError = !inputEl.classList.contains(InputInvalidClass);
            if (noError) {
                return;
            } else {
                // This case should occur when the form validation fails and an error occurs
                // without the user having interacted with the field.
                shouldDisplayErrorFeedback = true;
            }
        }

        const currentValue = e.currentTarget.value;
        errorHandler.checkField(feedBackDescription, currentValue);
    });

    inputEl.addEventListener('blur', (e) => {
        if (shouldDisplayErrorFeedback === false) {
            shouldDisplayErrorFeedback = true;
        } else {
            return;
        }

        const currentValue = e.currentTarget.value;
        errorHandler.checkField(feedBackDescription, currentValue);
    });
};
