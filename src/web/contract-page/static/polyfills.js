import smoothscroll from 'smoothscroll-polyfill';

export const polyfill = () => {
    // See https://developer.mozilla.org/en-US/docs/Web/API/Element/scrollIntoView.
    // Enables `scrollIntoViewOptions` with scrollIntoView().
    smoothscroll.polyfill();
}
