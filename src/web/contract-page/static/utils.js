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
