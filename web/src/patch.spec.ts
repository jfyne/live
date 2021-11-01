import { Patch } from "./patch";
import { LiveEvent } from "./event";

test("simple replace", () => {
    document.body.innerHTML = `<div _l0="">Hello</div>`;
    const event = new LiveEvent("patch", [
        {
            Anchor: "_l0",
            Action: 1,
            HTML: `<div _l0="">World</div>`,
        },
    ]);

    Patch.handle(event);
    expect(document.body.innerHTML).toEqual(`<div _l0="">World</div>`);
});

test("double update", () => {
    document.body.innerHTML = `<div _l0="">Hello</div><div _l1="">World</div>`;
    const p = new LiveEvent("patch", [
        {
            Anchor: "_l0",
            Action: 1,
            HTML: `<div _l0="">World</div>`,
        },
        {
            Anchor: "_l1",
            Action: 1,
            HTML: `<div _l1="">Hello</div>`,
        },
    ]);
    Patch.handle(p);
    expect(document.body.innerHTML).toEqual(`<div _l0="">World</div><div _l1="">Hello</div>`);
});

test("nested update", () => {
    document.body.innerHTML = `<form id="test" _l0=""><input type="text" _l01=""></form>`;
    const p = new LiveEvent("patch", [
        {
            Anchor: "_l0",
            Action: 3,
            HTML: `<div _l01="">Error</div>`,
        },
    ]);
    Patch.handle(p);

    expect(document.body.innerHTML).toEqual(
        `<form id="test" _l0=""><div _l01="">Error</div><input type="text" _l01=""></form>`
    );
});

test("head update", () => {
    document.head.innerHTML = `<title _l0="">1</title>`;
    const p = new LiveEvent("patch", [
        {
            Anchor: "_l0",
            Action: 1,
            HTML: `<title _l0="">2</title>`,
        },
    ]);
    Patch.handle(p);

    expect(document.head.innerHTML).toEqual(`<title _l0="">2</title>`);
});
