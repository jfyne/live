import { Patch } from "./patch";

test("simple replace", () => {
    document.body.innerHTML = "<div>Hello</div>";
    const event = {
        t: "patch",
        d: {
            Path: [1, 0],
            Action: 2,
            HTML: "<div>World</div>",
        },
    };

    Patch.handle(event);
    expect(document.body.innerHTML).toEqual("<div>World</div>");
});

test("double update", () => {
    document.body.innerHTML = "<div>Hello</div><div>World</div>";
    const one = {
        t: "patch",
        d: {
            Path: [1, 0],
            Action: 2,
            HTML: "<div>World</div>",
        },
    };
    Patch.handle(one);
    const two = {
        t: "patch",
        d: {
            Path: [1, 1],
            Action: 2,
            HTML: "<div>Hello</div>",
        },
    };
    Patch.handle(two);

    expect(document.body.innerHTML).toEqual("<div>World</div><div>Hello</div>");
});

test("nested update", () => {
    document.body.innerHTML = '<form><input type="text"></form>';
    const p = {
        t: "patch",
        d: {
            Path: [1, 0, 0],
            Action: 1,
            HTML: "<div>Error</div>",
        },
    };
    Patch.handle(p);

    expect(document.body.innerHTML).toEqual(
        '<form><div>Error</div><input type="text"></form>'
    );
});

test("head update", () => {
    document.head.innerHTML = "<title>1</title>";
    const p = {
        t: "patch",
        d: {
            Path: [0, 0],
            Action: 2,
            HTML: "<title>2</title>",
        },
    };
    Patch.handle(p);

    expect(document.head.innerHTML).toEqual("<title>2</title>");
});
