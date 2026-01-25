import { wireIslandEvents, EventWiring } from "./events";
import { ConnectionManager } from "./connection";

describe("Island-Scoped Event Wiring", () => {
    let islandElement: HTMLElement;
    let islandId: string;
    let cleanupFn: (() => void) | null = null;
    let mockSendEvent: jest.Mock;

    beforeEach(() => {
        // Create a fresh island element for each test
        islandElement = document.createElement("div");
        islandElement.setAttribute("data-island-id", "test-island");
        islandId = "test-island";
        document.body.appendChild(islandElement);

        // Mock the ConnectionManager sendEvent method
        mockSendEvent = jest.fn();
        const connectionManager = ConnectionManager.getInstance();
        jest.spyOn(connectionManager, "sendEvent").mockImplementation(mockSendEvent);
    });

    afterEach(() => {
        // Clean up event handlers
        if (cleanupFn) {
            cleanupFn();
            cleanupFn = null;
        }

        // Remove island from DOM
        if (islandElement.parentNode) {
            document.body.removeChild(islandElement);
        }

        // Clear all mocks
        jest.clearAllMocks();
    });

    describe("wireIslandEvents", () => {
        it("should return a cleanup function", () => {
            cleanupFn = wireIslandEvents(islandElement, islandId);
            expect(cleanupFn).toBeInstanceOf(Function);
        });

        it("should wire event handlers within island scope only", () => {
            // Create elements inside and outside the island
            islandElement.innerHTML = '<button live-click="inside">Inside</button>';
            const outsideButton = document.createElement("button");
            outsideButton.setAttribute("live-click", "outside");
            outsideButton.textContent = "Outside";
            document.body.appendChild(outsideButton);

            cleanupFn = wireIslandEvents(islandElement, islandId);

            // Click inside button should trigger event
            const insideButton = islandElement.querySelector("button");
            insideButton?.click();
            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "inside", {});

            // Click outside button should NOT trigger event
            mockSendEvent.mockClear();
            outsideButton.click();
            expect(mockSendEvent).not.toHaveBeenCalled();

            // Clean up
            document.body.removeChild(outsideButton);
        });
    });

    describe("Click Events", () => {
        it("should handle live-click events within island", () => {
            islandElement.innerHTML = '<button live-click="test-event">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");
            button?.click();

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "test-event", {});
        });

        it("should add loading class on click", () => {
            islandElement.innerHTML = '<button live-click="test-event">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button") as HTMLElement;
            button.click();

            expect(button.classList.contains("live-click-loading")).toBe(true);
        });

        it("should remove loading class on ack event", () => {
            islandElement.innerHTML = '<button live-click="test-event">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button") as HTMLElement;
            button.click();
            expect(button.classList.contains("live-click-loading")).toBe(true);

            // Simulate ACK event
            button.dispatchEvent(new Event("ack"));
            expect(button.classList.contains("live-click-loading")).toBe(false);
        });

        it("should include live-value attributes in event data", () => {
            islandElement.innerHTML = '<button live-click="test-event" live-value-id="123" live-value-action="edit">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");
            button?.click();

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "test-event", {
                id: "123",
                action: "edit",
            });
        });
    });

    describe("Focus and Blur Events", () => {
        it("should handle live-focus events", () => {
            islandElement.innerHTML = '<input live-focus="focus-event" />';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input");
            input?.dispatchEvent(new Event("focus"));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "focus-event", {});
        });

        it("should handle live-blur events", () => {
            islandElement.innerHTML = '<input live-blur="blur-event" />';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input");
            input?.dispatchEvent(new Event("blur"));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "blur-event", {});
        });
    });

    describe("Keyboard Events", () => {
        it("should handle live-keydown events", () => {
            islandElement.innerHTML = '<input live-keydown="keydown-event" />';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input");
            const keyEvent = new KeyboardEvent("keydown", { key: "Enter" });
            input?.dispatchEvent(keyEvent);

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "keydown-event", {
                key: "Enter",
                altKey: false,
                ctrlKey: false,
                shiftKey: false,
                metaKey: false,
            });
        });

        it("should handle live-keyup events", () => {
            islandElement.innerHTML = '<input live-keyup="keyup-event" />';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input");
            const keyEvent = new KeyboardEvent("keyup", { key: "a", ctrlKey: true });
            input?.dispatchEvent(keyEvent);

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "keyup-event", {
                key: "a",
                altKey: false,
                ctrlKey: true,
                shiftKey: false,
                metaKey: false,
            });
        });

        it("should filter key events with live-key attribute", () => {
            islandElement.innerHTML = '<input live-keydown="keydown-event" live-key="Enter" />';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input");

            // Press Enter - should trigger
            const enterEvent = new KeyboardEvent("keydown", { key: "Enter" });
            input?.dispatchEvent(enterEvent);
            expect(mockSendEvent).toHaveBeenCalledTimes(1);

            // Press 'a' - should NOT trigger
            const aEvent = new KeyboardEvent("keydown", { key: "a" });
            input?.dispatchEvent(aEvent);
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
        });
    });

    describe("Form Submit Events", () => {
        it("should handle live-submit events", () => {
            islandElement.innerHTML = `
                <form live-submit="submit-event">
                    <input name="username" value="john" />
                    <input name="email" value="john@example.com" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const form = islandElement.querySelector("form");
            const submitEvent = new Event("submit", { bubbles: true, cancelable: true });
            form?.dispatchEvent(submitEvent);

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "submit-event", {
                username: "john",
                email: "john@example.com",
            });
        });

        it("should prevent default form submission", () => {
            islandElement.innerHTML = `
                <form live-submit="submit-event">
                    <input name="username" value="john" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const form = islandElement.querySelector("form");
            const submitEvent = new Event("submit", { bubbles: true, cancelable: true });
            const preventDefaultSpy = jest.spyOn(submitEvent, "preventDefault");
            form?.dispatchEvent(submitEvent);

            expect(preventDefaultSpy).toHaveBeenCalled();
        });

        it("should add loading class on submit", () => {
            islandElement.innerHTML = `
                <form live-submit="submit-event">
                    <input name="username" value="john" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const form = islandElement.querySelector("form") as HTMLElement;
            form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

            expect(form.classList.contains("live-submit-loading")).toBe(true);
        });

        it("should merge form data with live-value attributes", () => {
            islandElement.innerHTML = `
                <form live-submit="submit-event" live-value-action="create">
                    <input name="username" value="john" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const form = islandElement.querySelector("form");
            form?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "submit-event", {
                action: "create",
                username: "john",
            });
        });
    });

    describe("Form Change Events", () => {
        it("should handle live-change events on input", () => {
            islandElement.innerHTML = `
                <form live-change="change-event">
                    <input name="username" value="john" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input") as HTMLInputElement;
            input.value = "jane";
            input.dispatchEvent(new Event("input", { bubbles: true }));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "change-event", {
                username: "jane",
            });
        });

        it("should handle live-change events on select", () => {
            islandElement.innerHTML = `
                <form live-change="change-event">
                    <select name="country">
                        <option value="us">US</option>
                        <option value="uk">UK</option>
                    </select>
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const select = islandElement.querySelector("select") as HTMLSelectElement;
            select.value = "uk";
            select.dispatchEvent(new Event("input", { bubbles: true }));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "change-event", {
                country: "uk",
            });
        });

        it("should handle live-change events on textarea", () => {
            islandElement.innerHTML = `
                <form live-change="change-event">
                    <textarea name="bio">Hello</textarea>
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const textarea = islandElement.querySelector("textarea") as HTMLTextAreaElement;
            textarea.value = "Hello World";
            textarea.dispatchEvent(new Event("input", { bubbles: true }));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "change-event", {
                bio: "Hello World",
            });
        });
    });

    describe("Debounce Functionality", () => {
        beforeEach(() => {
            jest.useFakeTimers();
        });

        afterEach(() => {
            jest.restoreAllMocks();
        });

        it("should debounce click events with live-debounce", () => {
            islandElement.innerHTML = '<button live-click="test-event" live-debounce="500">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");

            // Click multiple times rapidly
            button?.click();
            button?.click();
            button?.click();

            // Should not have sent yet
            expect(mockSendEvent).not.toHaveBeenCalled();

            // Fast-forward time
            jest.advanceTimersByTime(500);

            // Should have sent only once
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "test-event", {});
        });

        it("should debounce input events with live-debounce", () => {
            islandElement.innerHTML = `
                <form live-change="change-event">
                    <input name="search" live-debounce="300" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input") as HTMLInputElement;

            // Type rapidly
            input.value = "a";
            input.dispatchEvent(new Event("input", { bubbles: true }));
            input.value = "ab";
            input.dispatchEvent(new Event("input", { bubbles: true }));
            input.value = "abc";
            input.dispatchEvent(new Event("input", { bubbles: true }));

            // Should not have sent yet
            expect(mockSendEvent).not.toHaveBeenCalled();

            // Fast-forward time
            jest.advanceTimersByTime(300);

            // Should have sent only once with final value
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "change-event", {
                search: "abc",
            });
        });

        it("should support blur debounce mode", () => {
            islandElement.innerHTML = `
                <form live-change="change-event">
                    <input name="email" live-debounce="blur" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input") as HTMLInputElement;

            // Type value
            input.value = "test@example.com";
            input.dispatchEvent(new Event("input", { bubbles: true }));

            // Should not have sent yet
            expect(mockSendEvent).not.toHaveBeenCalled();

            // Trigger blur
            input.dispatchEvent(new Event("blur"));

            // Should have sent after blur
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "change-event", {
                email: "test@example.com",
            });
        });
    });

    describe("Event Isolation Between Islands", () => {
        let island2Element: HTMLElement;
        let island2Id: string;
        let cleanup2Fn: (() => void) | null = null;

        beforeEach(() => {
            // Create a second island
            island2Element = document.createElement("div");
            island2Element.setAttribute("data-island-id", "test-island-2");
            island2Id = "test-island-2";
            document.body.appendChild(island2Element);
        });

        afterEach(() => {
            if (cleanup2Fn) {
                cleanup2Fn();
                cleanup2Fn = null;
            }
            if (island2Element.parentNode) {
                document.body.removeChild(island2Element);
            }
        });

        it("should not trigger events from other islands", () => {
            // Set up two islands with similar elements
            islandElement.innerHTML = '<button live-click="island1-event">Island 1</button>';
            island2Element.innerHTML = '<button live-click="island2-event">Island 2</button>';

            cleanupFn = wireIslandEvents(islandElement, islandId);
            cleanup2Fn = wireIslandEvents(island2Element, island2Id);

            // Click island 1 button
            const island1Button = islandElement.querySelector("button");
            island1Button?.click();

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "island1-event", {});
            expect(mockSendEvent).not.toHaveBeenCalledWith(island2Id, expect.any(String), expect.any(Object));

            // Clear and click island 2 button
            mockSendEvent.mockClear();
            const island2Button = island2Element.querySelector("button");
            island2Button?.click();

            expect(mockSendEvent).toHaveBeenCalledWith(island2Id, "island2-event", {});
            expect(mockSendEvent).not.toHaveBeenCalledWith(islandId, expect.any(String), expect.any(Object));
        });

        it("should maintain separate loading states per island", () => {
            islandElement.innerHTML = '<button live-click="island1-event">Island 1</button>';
            island2Element.innerHTML = '<button live-click="island2-event">Island 2</button>';

            cleanupFn = wireIslandEvents(islandElement, islandId);
            cleanup2Fn = wireIslandEvents(island2Element, island2Id);

            const island1Button = islandElement.querySelector("button") as HTMLElement;
            const island2Button = island2Element.querySelector("button") as HTMLElement;

            // Click island 1 button
            island1Button.click();
            expect(island1Button.classList.contains("live-click-loading")).toBe(true);
            expect(island2Button.classList.contains("live-click-loading")).toBe(false);

            // Click island 2 button
            island2Button.click();
            expect(island1Button.classList.contains("live-click-loading")).toBe(true);
            expect(island2Button.classList.contains("live-click-loading")).toBe(true);

            // ACK island 1
            island1Button.dispatchEvent(new Event("ack"));
            expect(island1Button.classList.contains("live-click-loading")).toBe(false);
            expect(island2Button.classList.contains("live-click-loading")).toBe(true);
        });
    });

    describe("Cleanup", () => {
        it("should remove event listeners when cleanup is called", () => {
            islandElement.innerHTML = '<button live-click="test-event">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");
            button?.click();
            expect(mockSendEvent).toHaveBeenCalledTimes(1);

            // Call cleanup
            cleanupFn();
            cleanupFn = null;

            // Click again - should not trigger
            mockSendEvent.mockClear();
            button?.click();
            expect(mockSendEvent).not.toHaveBeenCalled();
        });

        it("should clear debounce timers on cleanup", () => {
            jest.useFakeTimers();

            islandElement.innerHTML = '<button live-click="test-event" live-debounce="500">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");
            button?.click();

            // Call cleanup before timer fires
            cleanupFn();
            cleanupFn = null;

            // Fast-forward time
            jest.advanceTimersByTime(500);

            // Should not have sent event
            expect(mockSendEvent).not.toHaveBeenCalled();

            jest.restoreAllMocks();
        });
    });

    describe("Mouse Events", () => {
        it("should handle live-contextmenu events", () => {
            islandElement.innerHTML = '<div live-contextmenu="context-event">Right Click Me</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const div = islandElement.querySelector("div");
            div?.dispatchEvent(new Event("contextmenu"));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "context-event", {});
        });

        it("should handle live-mousedown events", () => {
            islandElement.innerHTML = '<div live-mousedown="mousedown-event">Mouse Down</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const div = islandElement.querySelector("div");
            div?.dispatchEvent(new Event("mousedown"));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "mousedown-event", {});
        });

        it("should handle live-mouseup events", () => {
            islandElement.innerHTML = '<div live-mouseup="mouseup-event">Mouse Up</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const div = islandElement.querySelector("div");
            div?.dispatchEvent(new Event("mouseup"));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "mouseup-event", {});
        });
    });

    describe("Event Payload", () => {
        it("should always include island ID when sending events", () => {
            islandElement.innerHTML = '<button live-click="test-event">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");
            button?.click();

            // First argument should be island ID
            expect(mockSendEvent).toHaveBeenCalledWith(
                islandId,
                expect.any(String),
                expect.any(Object)
            );
        });

        it("should extract and include all form data", () => {
            islandElement.innerHTML = `
                <form live-submit="submit-event">
                    <input name="username" value="john" />
                    <input name="password" value="secret" />
                    <input name="remember" type="checkbox" checked />
                    <select name="role">
                        <option value="admin" selected>Admin</option>
                        <option value="user">User</option>
                    </select>
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const form = islandElement.querySelector("form");
            form?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "submit-event", {
                username: "john",
                password: "secret",
                remember: "on",
                role: "admin",
            });
        });
    });
});
