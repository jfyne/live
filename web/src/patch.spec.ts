import { applyIslandPatch, applyIslandPatches, Patch } from "./patch";
import { EventDispatch } from "./event";

// Mock EventDispatch
jest.mock("./event", () => ({
    EventDispatch: {
        beforeUpdate: jest.fn(),
        updated: jest.fn(),
        beforeDestroy: jest.fn(),
        destroyed: jest.fn(),
    },
}));

// Mock rewireEvents function
const mockRewireEvents = jest.fn();

describe("Island-scoped patch handling", () => {
    beforeEach(() => {
        document.body.innerHTML = "";
        jest.clearAllMocks();
    });

    describe("applyIslandPatch", () => {
        test("replaces element within island scope only", () => {
            // Create two islands
            const island1 = document.createElement("div");
            island1.id = "island1";
            island1.innerHTML = `<div _i_island1_0="">Hello Island 1</div>`;

            const island2 = document.createElement("div");
            island2.id = "island2";
            island2.innerHTML = `<div _i_island2_0="">Hello Island 2</div>`;

            document.body.appendChild(island1);
            document.body.appendChild(island2);

            // Patch island1 only
            const patch = {
                Anchor: "_i_island1_0",
                Action: 1, // REPLACE
                HTML: `<div _i_island1_0="">Updated Island 1</div>`,
            };

            applyIslandPatch(island1, patch, mockRewireEvents);

            // Island 1 should be updated
            expect(island1.innerHTML).toEqual(
                `<div _i_island1_0="">Updated Island 1</div>`
            );

            // Island 2 should be unchanged
            expect(island2.innerHTML).toEqual(
                `<div _i_island2_0="">Hello Island 2</div>`
            );

            // Events should be re-wired
            expect(mockRewireEvents).toHaveBeenCalledWith(island1);
        });

        test("appends element within island scope", () => {
            const island = document.createElement("div");
            island.innerHTML = `<div _i_test_0="">Container</div>`;
            document.body.appendChild(island);

            const patch = {
                Anchor: "_i_test_0",
                Action: 2, // APPEND
                HTML: `<span>Appended</span>`,
            };

            applyIslandPatch(island, patch, mockRewireEvents);

            expect(island.querySelector('[_i_test_0]')?.innerHTML).toContain(
                "Container"
            );
            expect(island.querySelector('[_i_test_0]')?.innerHTML).toContain(
                "Appended"
            );
            expect(mockRewireEvents).toHaveBeenCalledWith(island);
        });

        test("prepends element within island scope", () => {
            const island = document.createElement("div");
            island.innerHTML = `<div _i_test_0="">Container</div>`;
            document.body.appendChild(island);

            const patch = {
                Anchor: "_i_test_0",
                Action: 3, // PREPEND
                HTML: `<span>Prepended</span>`,
            };

            applyIslandPatch(island, patch, mockRewireEvents);

            const target = island.querySelector('[_i_test_0]');
            expect(target?.firstChild?.textContent).toBe("Prepended");
            expect(mockRewireEvents).toHaveBeenCalledWith(island);
        });

        test("ignores anchor not found in island", () => {
            const island = document.createElement("div");
            island.innerHTML = `<div _i_test_0="">Content</div>`;
            document.body.appendChild(island);

            const patch = {
                Anchor: "_i_other_0",
                Action: 1,
                HTML: `<div>Should not appear</div>`,
            };

            applyIslandPatch(island, patch, mockRewireEvents);

            expect(island.innerHTML).toEqual(
                `<div _i_test_0="">Content</div>`
            );
            expect(mockRewireEvents).not.toHaveBeenCalled();
        });

        test("handles noop action", () => {
            const island = document.createElement("div");
            island.innerHTML = `<div _i_test_0="">Content</div>`;
            document.body.appendChild(island);

            const patch = {
                Anchor: "_i_test_0",
                Action: 0, // NOOP
                HTML: `<div>Should not replace</div>`,
            };

            applyIslandPatch(island, patch, mockRewireEvents);

            expect(island.innerHTML).toEqual(
                `<div _i_test_0="">Content</div>`
            );
        });

        test("dispatches lifecycle events on replace", () => {
            const island = document.createElement("div");
            island.innerHTML = `<div _i_test_0="">Old</div>`;
            document.body.appendChild(island);

            const patch = {
                Anchor: "_i_test_0",
                Action: 1,
                HTML: `<div _i_test_0="">New</div>`,
            };

            applyIslandPatch(island, patch, mockRewireEvents);

            expect(EventDispatch.beforeUpdate).toHaveBeenCalled();
            expect(EventDispatch.updated).toHaveBeenCalled();
        });

        test("dispatches destroy events when replacing with empty", () => {
            const island = document.createElement("div");
            island.innerHTML = `<div _i_test_0="">To Remove</div>`;
            document.body.appendChild(island);

            const target = island.querySelector('[_i_test_0]');

            const patch = {
                Anchor: "_i_test_0",
                Action: 1,
                HTML: "",
            };

            applyIslandPatch(island, patch, mockRewireEvents);

            expect(EventDispatch.beforeDestroy).toHaveBeenCalledWith(target);
            expect(EventDispatch.destroyed).toHaveBeenCalledWith(target);
        });
    });

    describe("applyIslandPatches", () => {
        test("applies multiple patches to island", () => {
            const island = document.createElement("div");
            island.innerHTML = `
                <div _i_test_0="">First</div>
                <div _i_test_1="">Second</div>
            `;
            document.body.appendChild(island);

            const patches = [
                {
                    Anchor: "_i_test_0",
                    Action: 1,
                    HTML: `<div _i_test_0="">Updated First</div>`,
                },
                {
                    Anchor: "_i_test_1",
                    Action: 1,
                    HTML: `<div _i_test_1="">Updated Second</div>`,
                },
            ];

            applyIslandPatches(island, patches, mockRewireEvents);

            expect(island.innerHTML).toContain("Updated First");
            expect(island.innerHTML).toContain("Updated Second");
            expect(mockRewireEvents).toHaveBeenCalledTimes(2);
        });

        test("preserves form state within island during patches", () => {
            const island = document.createElement("div");
            island.innerHTML = `
                <form id="test-form" _i_test_0="">
                    <input type="text" name="username" value="john">
                    <input type="checkbox" name="agree">
                </form>
            `;
            document.body.appendChild(island);

            // Set form values
            const input = island.querySelector(
                '[name="username"]'
            ) as HTMLInputElement;
            input.value = "jane";

            const checkbox = island.querySelector(
                '[name="agree"]'
            ) as HTMLInputElement;
            checkbox.checked = true;

            const patches = [
                {
                    Anchor: "_i_test_0",
                    Action: 1,
                    HTML: `
                        <form id="test-form" _i_test_0="">
                            <input type="text" name="username" value="">
                            <input type="checkbox" name="agree">
                        </form>
                    `,
                },
            ];

            applyIslandPatches(island, patches, mockRewireEvents);

            // Form values should be preserved
            const updatedInput = island.querySelector(
                '[name="username"]'
            ) as HTMLInputElement;
            expect(updatedInput.value).toBe("jane");

            const updatedCheckbox = island.querySelector(
                '[name="agree"]'
            ) as HTMLInputElement;
            expect(updatedCheckbox.checked).toBe(true);
        });

        test("does not affect forms in other islands", () => {
            const island1 = document.createElement("div");
            island1.innerHTML = `
                <form id="form1" _i_island1_0="">
                    <input type="text" name="field1" value="original1">
                </form>
            `;

            const island2 = document.createElement("div");
            island2.innerHTML = `
                <form id="form2" _i_island2_0="">
                    <input type="text" name="field2" value="original2">
                </form>
            `;

            document.body.appendChild(island1);
            document.body.appendChild(island2);

            // Set form values
            const input1 = island1.querySelector(
                '[name="field1"]'
            ) as HTMLInputElement;
            input1.value = "changed1";

            const input2 = island2.querySelector(
                '[name="field2"]'
            ) as HTMLInputElement;
            input2.value = "changed2";

            // Patch island1 only
            const patches = [
                {
                    Anchor: "_i_island1_0",
                    Action: 1,
                    HTML: `
                        <form id="form1" _i_island1_0="">
                            <input type="text" name="field1" value="">
                        </form>
                    `,
                },
            ];

            applyIslandPatches(island1, patches, mockRewireEvents);

            // Island1 form should be preserved
            const updated1 = island1.querySelector(
                '[name="field1"]'
            ) as HTMLInputElement;
            expect(updated1.value).toBe("changed1");

            // Island2 form should be unchanged
            const updated2 = island2.querySelector(
                '[name="field2"]'
            ) as HTMLInputElement;
            expect(updated2.value).toBe("changed2");
        });

        test("handles nested elements correctly", () => {
            const island = document.createElement("div");
            island.innerHTML = `
                <div _i_test_0="">
                    <span _i_test_0_0="">Nested</span>
                </div>
            `;
            document.body.appendChild(island);

            const patches = [
                {
                    Anchor: "_i_test_0_0",
                    Action: 1,
                    HTML: `<span _i_test_0_0="">Updated Nested</span>`,
                },
            ];

            applyIslandPatches(island, patches, mockRewireEvents);

            expect(island.innerHTML).toContain("Updated Nested");
            expect(island.querySelector('[_i_test_0_0]')?.textContent).toBe(
                "Updated Nested"
            );
        });
    });

    describe("Island isolation", () => {
        test("patches do not leak between islands", () => {
            const island1 = document.createElement("div");
            island1.id = "island1";
            island1.innerHTML = `<div _i_island1_0="">Island 1</div>`;

            const island2 = document.createElement("div");
            island2.id = "island2";
            island2.innerHTML = `<div _i_island2_0="">Island 2</div>`;

            document.body.appendChild(island1);
            document.body.appendChild(island2);

            // Try to patch with wrong island ID
            const patch = {
                Anchor: "_i_island2_0",
                Action: 1,
                HTML: `<div _i_island2_0="">Hacked</div>`,
            };

            // Apply to island1 - should not find anchor
            applyIslandPatch(island1, patch, mockRewireEvents);

            // Island 1 unchanged
            expect(island1.innerHTML).toEqual(
                `<div _i_island1_0="">Island 1</div>`
            );

            // Island 2 unchanged
            expect(island2.innerHTML).toEqual(
                `<div _i_island2_0="">Island 2</div>`
            );
        });

        test("multiple islands can patch independently", () => {
            const island1 = document.createElement("div");
            island1.innerHTML = `<div _i_island1_0="">Original 1</div>`;

            const island2 = document.createElement("div");
            island2.innerHTML = `<div _i_island2_0="">Original 2</div>`;

            document.body.appendChild(island1);
            document.body.appendChild(island2);

            const patch1 = {
                Anchor: "_i_island1_0",
                Action: 1,
                HTML: `<div _i_island1_0="">Updated 1</div>`,
            };

            const patch2 = {
                Anchor: "_i_island2_0",
                Action: 1,
                HTML: `<div _i_island2_0="">Updated 2</div>`,
            };

            applyIslandPatches(island1, [patch1], mockRewireEvents);
            applyIslandPatches(island2, [patch2], mockRewireEvents);

            expect(island1.innerHTML).toContain("Updated 1");
            expect(island2.innerHTML).toContain("Updated 2");
        });
    });

    describe("Legacy Patch class (v1 compatibility)", () => {
        test("simple replace", () => {
            document.body.innerHTML = `<div _l0="">Hello</div>`;
            const event = {
                data: [
                    {
                        Anchor: "_l0",
                        Action: 1,
                        HTML: `<div _l0="">World</div>`,
                    },
                ],
            };

            Patch.handle(event);
            expect(document.body.innerHTML).toEqual(`<div _l0="">World</div>`);
        });

        test("double update", () => {
            document.body.innerHTML = `<div _l0="">Hello</div><div _l1="">World</div>`;
            const event = {
                data: [
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
                ],
            };

            Patch.handle(event);
            expect(document.body.innerHTML).toEqual(
                `<div _l0="">World</div><div _l1="">Hello</div>`
            );
        });

        test("nested update with prepend", () => {
            document.body.innerHTML = `<form id="test" _l0=""><input type="text" _l01=""></form>`;
            const event = {
                data: [
                    {
                        Anchor: "_l0",
                        Action: 3,
                        HTML: `<div _l01="">Error</div>`,
                    },
                ],
            };

            Patch.handle(event);

            expect(document.body.innerHTML).toEqual(
                `<form id="test" _l0=""><div _l01="">Error</div><input type="text" _l01=""></form>`
            );
        });
    });
});
