// Polyfill matchMedia for jsdom (missing in jsdom by default)
if (!window.matchMedia) {
    window.matchMedia = function () {
        return {
            matches: false,
            media: '',
            onchange: null,
            addListener: function () {},
            removeListener: function () {},
            addEventListener: function () {},
            removeEventListener: function () {},
            dispatchEvent: function () { return false; },
        };
    };
}
