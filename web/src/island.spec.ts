import { LiveIsland, registerLiveIsland } from "./island";
import { ConnectionManager } from "./connection";
import { IslandPatch, PatchAction } from "./transport/message";
import { Forms } from "./forms";
import { EventDispatch } from "./event";
import { HookRegistry } from "./hooks";

// Mock ConnectionManager
jest.mock("./connection");

// Mock Forms
jest.mock("./forms", () => ({
    Forms: {
        dehydrate: jest.fn(),
        hydrate: jest.fn(),
    },
}));

// Mock EventDispatch
jest.mock("./event", () => ({
    EventDispatch: {
        beforeUpdate: jest.fn(),
        updated: jest.fn(),
        beforeDestroy: jest.fn(),
        destroyed: jest.fn(),
    },
}));

// Mock HookRegistry
jest.mock("./hooks", () => ({
    HookRegistry: {
        executeElementHook: jest.fn(),
        cleanupIsland: jest.fn(),
    },
}));

describe("LiveIsland Custom Element", () => {
    let mockConnectionManager: jest.Mocked<ConnectionManager>;

    beforeEach(() => {
        // Clear mocks
        jest.clearAllMocks();

        // Mock ConnectionManager.getInstance
        mockConnectionManager = {
            registerIsland: jest.fn(),
            unregisterIsland: jest.fn(),
            sendEvent: jest.fn(),
            connect: jest.fn(),
            disconnect: jest.fn(),
            getState: jest.fn(),
            getRegisteredIslands: jest.fn(),
        } as any;

        (ConnectionManager.getInstance as jest.Mock).mockReturnValue(mockConnectionManager);

        // Ensure custom element is registered
        registerLiveIsland();
    });

    afterEach(() => {
        // Clean up DOM
        document.body.innerHTML = '';
    });

    describe("Custom Element Registration", () => {
        test("should register as 'live-island' custom element", () => {
            const element = document.createElement('live-island');
            expect(element).toBeInstanceOf(LiveIsland);
            expect(element.tagName.toLowerCase()).toBe('live-island');
        });

        test("should be an instance of HTMLElement", () => {
            const element = document.createElement('live-island');
            expect(element).toBeInstanceOf(HTMLElement);
        });

        test("should not register multiple times", () => {
            const spy = jest.spyOn(customElements, 'define');
            registerLiveIsland();
            registerLiveIsland();

            // Should only define once (initial registration before tests)
            expect(spy).not.toHaveBeenCalled();

            spy.mockRestore();
        });
    });

    describe("Props Extraction", () => {
        test("should extract type and id from attributes", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            document.body.appendChild(island);

            const props = island.getProps();
            expect(props).not.toBeNull();
            expect(props?.type).toBe('counter');
            expect(props?.id).toBe('counter-1');
        });

        test("should extract data-* attributes as camelCase props", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            island.setAttribute('data-initial-value', '10');
            island.setAttribute('data-max-count', '100');
            document.body.appendChild(island);

            const props = island.getProps();
            expect(props).not.toBeNull();
            expect(props?.initialValue).toBe('10');
            expect(props?.maxCount).toBe('100');
        });

        test("should handle multiple data-* attributes", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'user-profile');
            island.setAttribute('id', 'profile-1');
            island.setAttribute('data-user-id', '42');
            island.setAttribute('data-is-admin', 'true');
            island.setAttribute('data-theme-color', 'blue');
            document.body.appendChild(island);

            const props = island.getProps();
            expect(props).not.toBeNull();
            expect(props?.userId).toBe('42');
            expect(props?.isAdmin).toBe('true');
            expect(props?.themeColor).toBe('blue');
        });

        test("should handle no data-* attributes", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'simple');
            island.setAttribute('id', 'simple-1');
            document.body.appendChild(island);

            const props = island.getProps();
            expect(props).not.toBeNull();
            expect(props?.type).toBe('simple');
            expect(props?.id).toBe('simple-1');
            expect(Object.keys(props || {}).length).toBe(2); // Only type and id
        });
    });

    describe("connectedCallback", () => {
        test("should register with ConnectionManager on connect", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');

            document.body.appendChild(island);

            expect(mockConnectionManager.registerIsland).toHaveBeenCalledWith(
                'counter-1',
                'counter',
                expect.any(Function)
            );
        });

        test("should not register without id attribute", () => {
            const consoleSpy = jest.spyOn(console, 'error').mockImplementation();

            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            // No id attribute

            document.body.appendChild(island);

            expect(mockConnectionManager.registerIsland).not.toHaveBeenCalled();
            expect(consoleSpy).toHaveBeenCalledWith(
                expect.stringContaining('missing required "id" attribute'),
                expect.anything()
            );

            consoleSpy.mockRestore();
        });

        test("should not register without type attribute", () => {
            const consoleSpy = jest.spyOn(console, 'error').mockImplementation();

            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('id', 'counter-1');
            // No type attribute

            document.body.appendChild(island);

            expect(mockConnectionManager.registerIsland).not.toHaveBeenCalled();
            expect(consoleSpy).toHaveBeenCalledWith(
                expect.stringContaining('missing required "type" attribute'),
                expect.anything()
            );

            consoleSpy.mockRestore();
        });

        test("should extract props before registering", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            island.setAttribute('data-initial', '5');

            document.body.appendChild(island);

            const props = island.getProps();
            expect(props).not.toBeNull();
            expect(props?.type).toBe('counter');
            expect(props?.id).toBe('counter-1');
            expect(props?.initial).toBe('5');
        });
    });

    describe("disconnectedCallback", () => {
        test("should unregister from ConnectionManager on disconnect", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');

            document.body.appendChild(island);

            // Clear previous calls
            mockConnectionManager.unregisterIsland.mockClear();

            document.body.removeChild(island);

            expect(mockConnectionManager.unregisterIsland).toHaveBeenCalledWith('counter-1');
        });

        test("should cleanup props and islandId", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');

            document.body.appendChild(island);

            expect(island.getProps()).not.toBeNull();

            document.body.removeChild(island);

            expect(island.getProps()).toBeNull();
        });

        test("should handle disconnect without prior connect", () => {
            const island = document.createElement('live-island') as LiveIsland;
            // Don't append to DOM, just create

            // Should not throw
            expect(() => {
                island.disconnectedCallback();
            }).not.toThrow();
        });
    });

    describe("attributeChangedCallback", () => {
        test("should re-extract props when type changes", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            document.body.appendChild(island);

            let props = island.getProps();
            expect(props?.type).toBe('counter');

            island.setAttribute('type', 'timer');

            props = island.getProps();
            expect(props?.type).toBe('timer');
        });

        test("should re-register when id changes", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            document.body.appendChild(island);

            mockConnectionManager.registerIsland.mockClear();
            mockConnectionManager.unregisterIsland.mockClear();

            island.setAttribute('id', 'counter-2');

            // Should unregister old id and register new id
            expect(mockConnectionManager.unregisterIsland).toHaveBeenCalledWith('counter-1');
            expect(mockConnectionManager.registerIsland).toHaveBeenCalledWith(
                'counter-2',
                'counter',
                expect.any(Function)
            );
        });

        test("should not trigger callbacks before element is connected", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            // Not connected yet

            mockConnectionManager.registerIsland.mockClear();

            island.setAttribute('type', 'timer');

            // Should not register
            expect(mockConnectionManager.registerIsland).not.toHaveBeenCalled();
        });
    });

    describe("Patch Handling", () => {
        test("should receive and apply patches from ConnectionManager", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            island.innerHTML = '<div _i_counter-1_0>0</div>';
            document.body.appendChild(island);

            // Get the handler that was registered
            const handler = mockConnectionManager.registerIsland.mock.calls[0][2];

            // Simulate patch
            const patch: IslandPatch = {
                island_id: 'counter-1',
                patches: [
                    {
                        Anchor: '_i_counter-1_0',
                        Action: PatchAction.Replace,
                        HTML: '<div _i_counter-1_0>1</div>',
                    },
                ],
            };

            handler(patch);

            // Check that content was updated
            expect(island.innerHTML).toBe('<div _i_counter-1_0="">1</div>');
        });

        test("should preserve form state during patches", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'form');
            island.setAttribute('id', 'form-1');
            island.innerHTML = '<form id="test-form" _i_form-1_0><input name="test"></form>';
            document.body.appendChild(island);

            const handler = mockConnectionManager.registerIsland.mock.calls[0][2];

            const patch: IslandPatch = {
                island_id: 'form-1',
                patches: [
                    {
                        Anchor: '_i_form-1_0',
                        Action: PatchAction.Replace,
                        HTML: '<form id="test-form" _i_form-1_0><input name="test"></form>',
                    },
                ],
            };

            handler(patch);

            // Should call Forms.dehydrate before and Forms.hydrate after
            expect(Forms.dehydrate).toHaveBeenCalled();
            expect(Forms.hydrate).toHaveBeenCalled();
        });

        test("should handle Replace patch action", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'test');
            island.setAttribute('id', 'test-1');
            island.innerHTML = '<div _i_test-1_0>old</div>';
            document.body.appendChild(island);

            const handler = mockConnectionManager.registerIsland.mock.calls[0][2];

            const patch: IslandPatch = {
                island_id: 'test-1',
                patches: [
                    {
                        Anchor: '_i_test-1_0',
                        Action: PatchAction.Replace,
                        HTML: '<div _i_test-1_0>new</div>',
                    },
                ],
            };

            handler(patch);

            expect(island.innerHTML).toBe('<div _i_test-1_0="">new</div>');
            expect(EventDispatch.beforeUpdate).toHaveBeenCalled();
            expect(EventDispatch.updated).toHaveBeenCalled();
        });

        test("should handle Append patch action", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'test');
            island.setAttribute('id', 'test-1');
            island.innerHTML = '<div _i_test-1_0><span>existing</span></div>';
            document.body.appendChild(island);

            const handler = mockConnectionManager.registerIsland.mock.calls[0][2];

            const patch: IslandPatch = {
                island_id: 'test-1',
                patches: [
                    {
                        Anchor: '_i_test-1_0',
                        Action: PatchAction.Append,
                        HTML: '<span>appended</span>',
                    },
                ],
            };

            handler(patch);

            expect(island.innerHTML).toContain('<span>existing</span>');
            expect(island.innerHTML).toContain('<span>appended</span>');
        });

        test("should handle Prepend patch action", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'test');
            island.setAttribute('id', 'test-1');
            island.innerHTML = '<div _i_test-1_0><span>existing</span></div>';
            document.body.appendChild(island);

            const handler = mockConnectionManager.registerIsland.mock.calls[0][2];

            const patch: IslandPatch = {
                island_id: 'test-1',
                patches: [
                    {
                        Anchor: '_i_test-1_0',
                        Action: PatchAction.Prepend,
                        HTML: '<span>prepended</span>',
                    },
                ],
            };

            handler(patch);

            const div = island.querySelector('div');
            expect(div?.firstElementChild?.textContent).toBe('prepended');
        });

        test("should handle Noop patch action", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'test');
            island.setAttribute('id', 'test-1');
            island.innerHTML = '<div _i_test-1_0>unchanged</div>';
            document.body.appendChild(island);

            const handler = mockConnectionManager.registerIsland.mock.calls[0][2];

            const patch: IslandPatch = {
                island_id: 'test-1',
                patches: [
                    {
                        Anchor: '_i_test-1_0',
                        Action: PatchAction.Noop,
                        HTML: '',
                    },
                ],
            };

            handler(patch);

            expect(island.innerHTML).toBe('<div _i_test-1_0="">unchanged</div>');
        });

        test("should fallback to innerHTML when patch anchor not found", () => {
            const debugSpy = jest.spyOn(console, 'debug').mockImplementation();

            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'test');
            island.setAttribute('id', 'test-1');
            island.innerHTML = '<div>no anchor</div>';
            document.body.appendChild(island);

            const handler = mockConnectionManager.registerIsland.mock.calls[0][2];

            const patch: IslandPatch = {
                island_id: 'test-1',
                patches: [
                    {
                        Anchor: '_i_test-1_999',
                        Action: PatchAction.Replace,
                        HTML: '<div>test</div>',
                    },
                ],
            };

            handler(patch);

            // When anchor is not found and HTML is non-empty, the island
            // falls back to setting innerHTML (initial mount behavior)
            expect(island.innerHTML).toBe('<div>test</div>');

            debugSpy.mockRestore();
        });

        test("should handle multiple patches in sequence", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'test');
            island.setAttribute('id', 'test-1');
            island.innerHTML = '<div _i_test-1_0>0</div><div _i_test-1_1>1</div>';
            document.body.appendChild(island);

            const handler = mockConnectionManager.registerIsland.mock.calls[0][2];

            const patch: IslandPatch = {
                island_id: 'test-1',
                patches: [
                    {
                        Anchor: '_i_test-1_0',
                        Action: PatchAction.Replace,
                        HTML: '<div _i_test-1_0>updated-0</div>',
                    },
                    {
                        Anchor: '_i_test-1_1',
                        Action: PatchAction.Replace,
                        HTML: '<div _i_test-1_1>updated-1</div>',
                    },
                ],
            };

            handler(patch);

            expect(island.innerHTML).toContain('updated-0');
            expect(island.innerHTML).toContain('updated-1');
        });
    });

    describe("Multiple Islands on Same Page", () => {
        test("should allow multiple islands with different ids", () => {
            const island1 = document.createElement('live-island') as LiveIsland;
            island1.setAttribute('type', 'counter');
            island1.setAttribute('id', 'counter-1');
            island1.innerHTML = '<div _i_counter-1_0>0</div>';

            const island2 = document.createElement('live-island') as LiveIsland;
            island2.setAttribute('type', 'counter');
            island2.setAttribute('id', 'counter-2');
            island2.innerHTML = '<div _i_counter-2_0>0</div>';

            document.body.appendChild(island1);
            document.body.appendChild(island2);

            expect(mockConnectionManager.registerIsland).toHaveBeenCalledTimes(2);
            expect(mockConnectionManager.registerIsland).toHaveBeenCalledWith(
                'counter-1',
                'counter',
                expect.any(Function)
            );
            expect(mockConnectionManager.registerIsland).toHaveBeenCalledWith(
                'counter-2',
                'counter',
                expect.any(Function)
            );
        });

        test("should route patches to correct island", () => {
            const island1 = document.createElement('live-island') as LiveIsland;
            island1.setAttribute('type', 'counter');
            island1.setAttribute('id', 'counter-1');
            island1.innerHTML = '<div _i_counter-1_0>0</div>';

            const island2 = document.createElement('live-island') as LiveIsland;
            island2.setAttribute('type', 'counter');
            island2.setAttribute('id', 'counter-2');
            island2.innerHTML = '<div _i_counter-2_0>0</div>';

            document.body.appendChild(island1);
            document.body.appendChild(island2);

            // Get handlers
            const handler1 = mockConnectionManager.registerIsland.mock.calls[0][2];
            const handler2 = mockConnectionManager.registerIsland.mock.calls[1][2];

            // Patch island1
            const patch1: IslandPatch = {
                island_id: 'counter-1',
                patches: [
                    {
                        Anchor: '_i_counter-1_0',
                        Action: PatchAction.Replace,
                        HTML: '<div _i_counter-1_0>1</div>',
                    },
                ],
            };

            handler1(patch1);

            // Only island1 should be updated
            expect(island1.innerHTML).toBe('<div _i_counter-1_0="">1</div>');
            expect(island2.innerHTML).toBe('<div _i_counter-2_0="">0</div>');

            // Patch island2
            const patch2: IslandPatch = {
                island_id: 'counter-2',
                patches: [
                    {
                        Anchor: '_i_counter-2_0',
                        Action: PatchAction.Replace,
                        HTML: '<div _i_counter-2_0>2</div>',
                    },
                ],
            };

            handler2(patch2);

            // Now island2 should be updated
            expect(island1.innerHTML).toBe('<div _i_counter-1_0="">1</div>');
            expect(island2.innerHTML).toBe('<div _i_counter-2_0="">2</div>');
        });

        test("should cleanup all islands independently", () => {
            const island1 = document.createElement('live-island') as LiveIsland;
            island1.setAttribute('type', 'counter');
            island1.setAttribute('id', 'counter-1');

            const island2 = document.createElement('live-island') as LiveIsland;
            island2.setAttribute('type', 'counter');
            island2.setAttribute('id', 'counter-2');

            document.body.appendChild(island1);
            document.body.appendChild(island2);

            mockConnectionManager.unregisterIsland.mockClear();

            document.body.removeChild(island1);

            expect(mockConnectionManager.unregisterIsland).toHaveBeenCalledWith('counter-1');
            expect(mockConnectionManager.unregisterIsland).not.toHaveBeenCalledWith('counter-2');

            document.body.removeChild(island2);

            expect(mockConnectionManager.unregisterIsland).toHaveBeenCalledWith('counter-2');
        });
    });

    describe("sendEvent Method", () => {
        test("should send event via ConnectionManager", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            document.body.appendChild(island);

            island.sendEvent('click', { action: 'increment' });

            expect(mockConnectionManager.sendEvent).toHaveBeenCalledWith(
                'counter-1',
                'click',
                { action: 'increment' }
            );
        });

        test("should warn when sending event without island ID", () => {
            const consoleSpy = jest.spyOn(console, 'warn').mockImplementation();

            const island = document.createElement('live-island') as LiveIsland;
            // No id attribute

            island.sendEvent('click', {});

            expect(mockConnectionManager.sendEvent).not.toHaveBeenCalled();
            expect(consoleSpy).toHaveBeenCalledWith(
                expect.stringContaining('cannot send event without island ID')
            );

            consoleSpy.mockRestore();
        });

        test("should send event with no data", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            document.body.appendChild(island);

            island.sendEvent('reset');

            expect(mockConnectionManager.sendEvent).toHaveBeenCalledWith(
                'counter-1',
                'reset',
                undefined
            );
        });
    });

    describe("Hook Integration", () => {
        beforeEach(() => {
            // Clear hook registry mock calls
            (HookRegistry.executeElementHook as jest.Mock).mockClear();
            (HookRegistry.cleanupIsland as jest.Mock).mockClear();
        });

        test("should execute mounted hooks when island is connected", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');

            // Add an element with a hook
            const hookEl = document.createElement('div');
            hookEl.setAttribute('live-hook', 'TestHook');
            island.appendChild(hookEl);

            document.body.appendChild(island);

            // Should execute mounted hook for the element
            expect(HookRegistry.executeElementHook).toHaveBeenCalledWith(
                hookEl,
                island,
                'mounted'
            );
        });

        test("should execute updated hooks when patch is applied", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            island.innerHTML = '<div _i_counter-1_0="">Content</div>';

            // Add element with hook
            const hookEl = document.createElement('span');
            hookEl.setAttribute('live-hook', 'TestHook');
            island.appendChild(hookEl);

            document.body.appendChild(island);

            // Clear the mounted call
            (HookRegistry.executeElementHook as jest.Mock).mockClear();

            // Get the patch handler
            const registerCall = mockConnectionManager.registerIsland.mock.calls.find(
                call => call[0] === 'counter-1'
            );
            expect(registerCall).toBeDefined();
            const handler = registerCall![2];

            // Apply a patch
            const patch: IslandPatch = {
                island_id: 'counter-1',
                patches: [
                    {
                        Action: PatchAction.Replace,
                        Anchor: '_i_counter-1_0',
                        HTML: '<div _i_counter-1_0="">Updated</div>'
                    }
                ]
            };

            handler(patch);

            // Should execute updated hook
            expect(HookRegistry.executeElementHook).toHaveBeenCalledWith(
                hookEl,
                island,
                'updated'
            );
        });

        test("should execute destroyed hooks when island is disconnected", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');

            // Add element with hook
            const hookEl = document.createElement('div');
            hookEl.setAttribute('live-hook', 'TestHook');
            island.appendChild(hookEl);

            document.body.appendChild(island);

            // Clear mounted calls
            (HookRegistry.executeElementHook as jest.Mock).mockClear();

            // Remove the island
            document.body.removeChild(island);

            // Should execute destroyed hook
            expect(HookRegistry.executeElementHook).toHaveBeenCalledWith(
                hookEl,
                island,
                'destroyed'
            );
        });

        test("should cleanup island hooks when disconnected", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');
            document.body.appendChild(island);

            // Remove the island
            document.body.removeChild(island);

            // Should cleanup hook registry
            expect(HookRegistry.cleanupIsland).toHaveBeenCalledWith('counter-1');
        });

        test("should execute hooks for multiple elements", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');

            const hookEl1 = document.createElement('div');
            hookEl1.setAttribute('live-hook', 'Hook1');
            const hookEl2 = document.createElement('div');
            hookEl2.setAttribute('live-hook', 'Hook2');

            island.appendChild(hookEl1);
            island.appendChild(hookEl2);

            document.body.appendChild(island);

            // Should execute mounted hooks for both elements
            expect(HookRegistry.executeElementHook).toHaveBeenCalledWith(
                hookEl1,
                island,
                'mounted'
            );
            expect(HookRegistry.executeElementHook).toHaveBeenCalledWith(
                hookEl2,
                island,
                'mounted'
            );
        });

        test("should not execute hooks for elements without live-hook attribute", () => {
            const island = document.createElement('live-island') as LiveIsland;
            island.setAttribute('type', 'counter');
            island.setAttribute('id', 'counter-1');

            const normalEl = document.createElement('div');
            island.appendChild(normalEl);

            document.body.appendChild(island);

            // Should only be called for elements with live-hook attribute (none in this case)
            // querySelectorAll('[live-hook]') should return empty
            // So executeElementHook should not be called
            expect(HookRegistry.executeElementHook).not.toHaveBeenCalled();
        });
    });
});
