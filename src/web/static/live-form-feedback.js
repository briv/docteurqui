export const makeElement = (kind, f) => {
    const el = document.createElement(kind);
    if (typeof f === 'function') {
        f(el);
    }
    return el;
};
