import { Patch } from "./patch";
import { LiveEvent } from "./event";

test("simple replace", () => {
    document.body.innerHTML = "<div>Hello</div>";
    const event = new LiveEvent("patch", [
        {
            Path: [1, 0],
            Action: 2,
            HTML: "<div>World</div>",
        },
    ]);

    Patch.handle(event);
    expect(document.body.innerHTML).toEqual("<div>World</div>");
});

test("double update", () => {
    document.body.innerHTML = "<div>Hello</div><div>World</div>";
    const p = new LiveEvent("patch", [
        {
            Path: [1, 0],
            Action: 2,
            HTML: "<div>World</div>",
        },
        {
            Path: [1, 1],
            Action: 2,
            HTML: "<div>Hello</div>",
        },
    ]);
    Patch.handle(p);
    expect(document.body.innerHTML).toEqual("<div>World</div><div>Hello</div>");
});

test("nested update", () => {
    document.body.innerHTML = '<form id="test"><input type="text"></form>';
    const p = new LiveEvent("patch", [
        {
            Path: [1, 0, 0],
            Action: 1,
            HTML: "<div>Error</div>",
        },
    ]);
    Patch.handle(p);

    expect(document.body.innerHTML).toEqual(
        '<form id="test"><div>Error</div><input type="text"></form>'
    );
});

test("head update", () => {
    document.head.innerHTML = "<title>1</title>";
    const p = new LiveEvent("patch", [
        {
            Path: [0, 0],
            Action: 2,
            HTML: "<title>2</title>",
        },
    ]);
    Patch.handle(p);

    expect(document.head.innerHTML).toEqual("<title>2</title>");
});
