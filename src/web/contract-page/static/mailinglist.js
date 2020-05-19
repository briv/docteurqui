import { makeElement } from "./utils";

const processFormData = (form) => {
    const params = new URLSearchParams(new FormData(form));
    if (params.get('future-news-email') === '') {
        return null;
    }
    return params;
};

const showFeedback = (form, text) => {
    const emailContainer = form.querySelector('.email-container');

    let feedback = form.querySelector('.feedback');
    if (!feedback) {
        feedback = makeElement('div', el => {
            el.classList.add('feedback');
        });
        form.insertBefore(feedback, emailContainer.nextSibling);
    }
    feedback.textContent = text;
};

const WaitTime = 1500;
const ErrMessage = "Une erreur s'est produite, réessayez plus tard s'il vous plaît.";

const handleResponse = (form, response) => {
    const [textToShow, resetForm] = (() => {
        if (response.ok) {
            return ['Merci de votre soutien !', true];
        } else if (response.status >= 400 && response.status < 500) {
            return ['Erreur, veuillez vérifier votre email.', false];
        } else if (response.status >= 500) {
            return [ErrMessage, false];
        }
    })();

    if (resetForm) {
        form.reset();
    }
    showFeedback(form, textToShow);

    return new Promise((resolve, reject) => {
        setTimeout(() => {
            resolve();
        }, WaitTime);
    });
};

const handleError = (form, err) => {
    showFeedback(form, ErrMessage);

    return new Promise((resolve, reject) => {
        setTimeout(() => {
            resolve();
        }, WaitTime);
    });
};

export const Handlers = {
    processFormData,
    handleResponse,
    handleError,
};
