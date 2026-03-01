import { HookRegistry, IslandHooks, HookContext, autoRegisterHooks } from "./hooks";
import { LiveIsland } from "./island";

// Mock LiveIsland
jest.mock("./island");

describe("HookRegistry", () => {
    let mockIsland: jest.Mocked<LiveIsland>;
    let mockElement: HTMLElement;

    beforeEach(() => {
        // Clear hooks before each test
        HookRegistry.clear();

        // Create mock island
        mockIsland = {
            sendEvent: jest.fn(),
            getProps: jest.fn().mockReturnValue({ id: 'test-island-1', type: 'test' }),
        } as any;

        // Create mock element
        mockElement = document.createElement('div');
        mockElement.setAttribute('live-hook', 'TestHook');
        document.body.appendChild(mockElement);
    });

    afterEach(() => {
        document.body.innerHTML = '';
    });

    describe("Hook Registration", () => {
        test("should register hooks", () => {
            const hooks: IslandHooks = {
                TestHook: {
                    mounted() {
                        console.log('mounted');
                    }
                }
            };

            HookRegistry.register(hooks);

            expect(HookRegistry.getHook('TestHook')).toBeDefined();
            expect(HookRegistry.getRegisteredHooks()).toContain('TestHook');
        });

        test("should register multiple hooks", () => {
            const hooks: IslandHooks = {
                Hook1: { mounted() {} },
                Hook2: { updated() {} },
                Hook3: { destroyed() {} }
            };

            HookRegistry.register(hooks);

            expect(HookRegistry.getRegisteredHooks()).toEqual(['Hook1', 'Hook2', 'Hook3']);
        });

        test("should merge hooks when registered multiple times", () => {
            HookRegistry.register({
                Hook1: { mounted() {} }
            });

            HookRegistry.register({
                Hook2: { updated() {} }
            });

            const registered = HookRegistry.getRegisteredHooks();
            expect(registered).toContain('Hook1');
            expect(registered).toContain('Hook2');
        });

        test("should return undefined for unregistered hook", () => {
            expect(HookRegistry.getHook('NonExistent')).toBeUndefined();
        });
    });

    describe("Hook Execution", () => {
        test("should execute mounted hook with context", () => {
            const mountedSpy = jest.fn();

            HookRegistry.register({
                TestHook: {
                    mounted: mountedSpy
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');

            expect(mountedSpy).toHaveBeenCalledTimes(1);
        });

        test("should provide correct context to hooks", () => {
            let capturedContext: HookContext | null = null;

            HookRegistry.register({
                TestHook: {
                    mounted(this: HookContext) {
                        capturedContext = this;
                    }
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');

            expect(capturedContext).not.toBeNull();
            expect(capturedContext!.el).toBe(mockElement);
            expect(capturedContext!.island).toBe(mockIsland);
            expect(typeof capturedContext!.pushEvent).toBe('function');
            expect(typeof capturedContext!.handleEvent).toBe('function');
        });

        test("should execute updated hook", () => {
            const updatedSpy = jest.fn();

            HookRegistry.register({
                TestHook: {
                    updated: updatedSpy
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'updated');

            expect(updatedSpy).toHaveBeenCalledTimes(1);
        });

        test("should execute destroyed hook", () => {
            const destroyedSpy = jest.fn();

            HookRegistry.register({
                TestHook: {
                    destroyed: destroyedSpy
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'destroyed');

            expect(destroyedSpy).toHaveBeenCalledTimes(1);
        });

        test("should handle missing lifecycle method gracefully", () => {
            HookRegistry.register({
                TestHook: {
                    mounted() {}
                    // No updated method
                }
            });

            expect(() => {
                HookRegistry.executeElementHook(mockElement, mockIsland, 'updated');
            }).not.toThrow();
        });

        test("should handle missing hook gracefully", () => {
            expect(() => {
                HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');
            }).not.toThrow();
        });

        test("should handle elements without live-hook attribute", () => {
            const element = document.createElement('div');
            document.body.appendChild(element);

            expect(() => {
                HookRegistry.executeElementHook(element, mockIsland, 'mounted');
            }).not.toThrow();
        });

        test("should catch and log errors in hooks", () => {
            const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation();

            HookRegistry.register({
                TestHook: {
                    mounted() {
                        throw new Error('Test error');
                    }
                }
            });

            expect(() => {
                HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');
            }).not.toThrow();

            expect(consoleErrorSpy).toHaveBeenCalled();
            consoleErrorSpy.mockRestore();
        });
    });

    describe("pushEvent", () => {
        test("should send event to island", () => {
            HookRegistry.register({
                TestHook: {
                    mounted(this: HookContext) {
                        this.pushEvent('test-event', { data: 'value' });
                    }
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');

            expect(mockIsland.sendEvent).toHaveBeenCalledWith('test-event', { data: 'value' });
        });

        test("should send event without data", () => {
            HookRegistry.register({
                TestHook: {
                    mounted(this: HookContext) {
                        this.pushEvent('test-event');
                    }
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');

            expect(mockIsland.sendEvent).toHaveBeenCalledWith('test-event', undefined);
        });

        test("should bind pushEvent to correct island", () => {
            const island1 = {
                sendEvent: jest.fn(),
                getProps: jest.fn().mockReturnValue({ id: 'island-1', type: 'test' })
            } as any;

            const island2 = {
                sendEvent: jest.fn(),
                getProps: jest.fn().mockReturnValue({ id: 'island-2', type: 'test' })
            } as any;

            HookRegistry.register({
                TestHook: {
                    mounted(this: HookContext) {
                        this.pushEvent('test-event');
                    }
                }
            });

            const element1 = document.createElement('div');
            element1.setAttribute('live-hook', 'TestHook');
            const element2 = document.createElement('div');
            element2.setAttribute('live-hook', 'TestHook');

            HookRegistry.executeElementHook(element1, island1, 'mounted');
            HookRegistry.executeElementHook(element2, island2, 'mounted');

            expect(island1.sendEvent).toHaveBeenCalledWith('test-event', undefined);
            expect(island2.sendEvent).toHaveBeenCalledWith('test-event', undefined);
        });
    });

    describe("handleEvent", () => {
        test("should register event handler", () => {
            const handler = jest.fn();

            HookRegistry.register({
                TestHook: {
                    mounted(this: HookContext) {
                        this.handleEvent('server-event', handler);
                    }
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');

            // Trigger server event
            HookRegistry.handleServerEvent('test-island-1', 'server-event', { data: 'test' });

            expect(handler).toHaveBeenCalledWith({ data: 'test' });
        });

        test("should register multiple handlers for same event", () => {
            const handler1 = jest.fn();
            const handler2 = jest.fn();

            HookRegistry.register({
                TestHook: {
                    mounted(this: HookContext) {
                        this.handleEvent('server-event', handler1);
                        this.handleEvent('server-event', handler2);
                    }
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');

            HookRegistry.handleServerEvent('test-island-1', 'server-event', { data: 'test' });

            expect(handler1).toHaveBeenCalledWith({ data: 'test' });
            expect(handler2).toHaveBeenCalledWith({ data: 'test' });
        });

        test("should isolate event handlers by island", () => {
            const handler1 = jest.fn();
            const handler2 = jest.fn();

            const island1 = {
                sendEvent: jest.fn(),
                getProps: jest.fn().mockReturnValue({ id: 'island-1', type: 'test' })
            } as any;

            const island2 = {
                sendEvent: jest.fn(),
                getProps: jest.fn().mockReturnValue({ id: 'island-2', type: 'test' })
            } as any;

            HookRegistry.register({
                TestHook: {
                    mounted(this: HookContext) {
                        this.handleEvent('server-event', this.island === island1 ? handler1 : handler2);
                    }
                }
            });

            const element1 = document.createElement('div');
            element1.setAttribute('live-hook', 'TestHook');
            const element2 = document.createElement('div');
            element2.setAttribute('live-hook', 'TestHook');

            HookRegistry.executeElementHook(element1, island1, 'mounted');
            HookRegistry.executeElementHook(element2, island2, 'mounted');

            // Trigger event for island-1 only
            HookRegistry.handleServerEvent('island-1', 'server-event', { data: 'test' });

            expect(handler1).toHaveBeenCalledWith({ data: 'test' });
            expect(handler2).not.toHaveBeenCalled();
        });

        test("should handle errors in event handlers gracefully", () => {
            const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation();

            const handler = jest.fn(() => {
                throw new Error('Handler error');
            });

            HookRegistry.register({
                TestHook: {
                    mounted(this: HookContext) {
                        this.handleEvent('server-event', handler);
                    }
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');

            expect(() => {
                HookRegistry.handleServerEvent('test-island-1', 'server-event', {});
            }).not.toThrow();

            expect(handler).toHaveBeenCalled();
            expect(consoleErrorSpy).toHaveBeenCalled();
            consoleErrorSpy.mockRestore();
        });
    });

    describe("Island Lifecycle", () => {
        test("should support all hook lifecycle methods", () => {
            const mounted = jest.fn();
            const beforeUpdate = jest.fn();
            const updated = jest.fn();
            const beforeDestroy = jest.fn();
            const destroyed = jest.fn();
            const disconnected = jest.fn();
            const reconnected = jest.fn();

            HookRegistry.register({
                TestHook: {
                    mounted,
                    beforeUpdate,
                    updated,
                    beforeDestroy,
                    destroyed,
                    disconnected,
                    reconnected
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');
            HookRegistry.executeElementHook(mockElement, mockIsland, 'beforeUpdate');
            HookRegistry.executeElementHook(mockElement, mockIsland, 'updated');
            HookRegistry.executeElementHook(mockElement, mockIsland, 'beforeDestroy');
            HookRegistry.executeElementHook(mockElement, mockIsland, 'destroyed');
            HookRegistry.executeElementHook(mockElement, mockIsland, 'disconnected');
            HookRegistry.executeElementHook(mockElement, mockIsland, 'reconnected');

            expect(mounted).toHaveBeenCalledTimes(1);
            expect(beforeUpdate).toHaveBeenCalledTimes(1);
            expect(updated).toHaveBeenCalledTimes(1);
            expect(beforeDestroy).toHaveBeenCalledTimes(1);
            expect(destroyed).toHaveBeenCalledTimes(1);
            expect(disconnected).toHaveBeenCalledTimes(1);
            expect(reconnected).toHaveBeenCalledTimes(1);
        });
    });

    describe("Cleanup", () => {
        test("should cleanup island event handlers", () => {
            const handler = jest.fn();

            HookRegistry.register({
                TestHook: {
                    mounted(this: HookContext) {
                        this.handleEvent('server-event', handler);
                    }
                }
            });

            HookRegistry.executeElementHook(mockElement, mockIsland, 'mounted');

            // Cleanup
            HookRegistry.cleanupIsland('test-island-1');

            // Event should not be handled
            HookRegistry.handleServerEvent('test-island-1', 'server-event', {});
            expect(handler).not.toHaveBeenCalled();
        });

        test("should clear all hooks", () => {
            HookRegistry.register({
                Hook1: { mounted() {} },
                Hook2: { updated() {} }
            });

            expect(HookRegistry.getRegisteredHooks().length).toBe(2);

            HookRegistry.clear();

            expect(HookRegistry.getRegisteredHooks().length).toBe(0);
        });
    });

    describe("Window.Hooks Auto-Registration", () => {
        test("should auto-register window.Hooks", () => {
            (window as any).Hooks = {
                WindowHook: {
                    mounted() {}
                }
            };

            autoRegisterHooks();

            expect(HookRegistry.getHook('WindowHook')).toBeDefined();
            expect(HookRegistry.getRegisteredHooks()).toContain('WindowHook');

            // Cleanup
            delete (window as any).Hooks;
        });

        test("should handle missing window.Hooks gracefully", () => {
            delete (window as any).Hooks;

            expect(() => {
                autoRegisterHooks();
            }).not.toThrow();
        });
    });
});
