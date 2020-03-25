export const makeElement = (kind, f) => {
    const el = document.createElement(kind);
    if (typeof f === 'function') {
        f(el);
    }
    return el;
};

const DefaultOptions = { behavior: 'smooth', block: 'start', inline: 'nearest' };
export const smoothScrollTo = (element, options = {}) => {
    element.scrollIntoView({ ...DefaultOptions, ...{
        block: options.block,
    } });
};

export class GenericUserError extends Error {
    constructor(userFacingMessage, technicalErrorMessage) {
        super(userFacingMessage);
        this.name = 'GenericUserError';
        this.technicalErrorMessage = technicalErrorMessage;
    }
}

export class FormValidationError extends Error {
    constructor(issues) {
        super('form validation failed');
        this.name = 'FormValidationError';
        this.issues = issues;
    }
}
