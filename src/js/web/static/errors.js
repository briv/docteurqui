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
