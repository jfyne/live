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

    // ---------------------------------------------------------------------------
    // Throttle Functionality
    // ---------------------------------------------------------------------------
    // These tests are RED: Throttler class and live-throttle support do not exist yet.
    // ---------------------------------------------------------------------------

    describe("Throttle Functionality", () => {
        beforeEach(() => {
            jest.useFakeTimers();
        });

        afterEach(() => {
            jest.useRealTimers();
        });

        it("should fire immediately on the first click with live-throttle", () => {
            // Scenario: Throttle fires immediately then rate-limits
            // Given an element with live-click="test" and live-throttle="500"
            // When the user clicks the element
            // Then the first click fires immediately
            islandElement.innerHTML = '<button live-click="throttle-event" live-throttle="500">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");
            button?.click();

            // First click should fire immediately (no timer needed)
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "throttle-event", {});
        });

        it("should rate-limit subsequent clicks within the throttle interval", () => {
            // Scenario: Throttle fires immediately then rate-limits
            // Given an element with live-click="test" and live-throttle="500"
            // When the user clicks the element 5 times rapidly
            // Then the first click fires immediately
            // And no further clicks fire until 500ms have elapsed
            islandElement.innerHTML = '<button live-click="throttle-event" live-throttle="500">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");

            // Click 5 times rapidly
            button?.click();
            button?.click();
            button?.click();
            button?.click();
            button?.click();

            // Only the first click should have fired immediately
            expect(mockSendEvent).toHaveBeenCalledTimes(1);

            // Advance time partway -- still within throttle window, no additional fires
            jest.advanceTimersByTime(400);
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
        });

        it("should fire a trailing event after the throttle interval elapses", () => {
            // Scenario: Throttle fires immediately then rate-limits
            // And a trailing fire occurs after the throttle interval
            islandElement.innerHTML = '<button live-click="throttle-event" live-throttle="500">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");

            // Click multiple times
            button?.click();
            button?.click();
            button?.click();

            // First click fired immediately
            expect(mockSendEvent).toHaveBeenCalledTimes(1);

            // After 500ms, the trailing fire should happen
            jest.advanceTimersByTime(500);
            expect(mockSendEvent).toHaveBeenCalledTimes(2);
        });

        it("should apply throttle over debounce when both attributes are present", () => {
            // Scenario: Throttle takes precedence over debounce
            // Given an element with live-throttle="500" and live-debounce="200"
            // When the user clicks the element
            // Then throttle behavior is applied, not debounce
            islandElement.innerHTML = '<button live-click="throttle-event" live-throttle="500" live-debounce="200">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");
            button?.click();

            // With throttle precedence: first click fires immediately (throttle behavior)
            // If debounce took precedence: first click would NOT fire immediately
            expect(mockSendEvent).toHaveBeenCalledTimes(1);

            // Advance past debounce but not throttle -- should still only be 1 call
            jest.advanceTimersByTime(200);
            // debounce would have fired here if it had won; throttle prevents it
            // (the trailing fire is at 500ms, not 200ms)
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
        });

        it("should not fire trailing throttle events after island cleanup", () => {
            // Scenario: Throttle cleanup on island unmount
            // Given an element with live-throttle="500" in an island
            // When the island is unmounted
            // Then no trailing throttle fires occur
            islandElement.innerHTML = '<button live-click="throttle-event" live-throttle="500">Click Me</button>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const button = islandElement.querySelector("button");

            // Click multiple times to arm a trailing fire
            button?.click();
            button?.click();
            expect(mockSendEvent).toHaveBeenCalledTimes(1);

            // Clean up before trailing timer fires
            cleanupFn();
            cleanupFn = null;

            // Advance past the throttle interval
            jest.advanceTimersByTime(500);

            // No trailing fire should occur after cleanup
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
        });

        it("should throttle keydown events with live-throttle", () => {
            // Scenario: Throttle with different event types (keydown)
            islandElement.innerHTML = '<input live-keydown="keydown-throttle" live-throttle="300" />';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input");

            // Fire keydown events rapidly
            input?.dispatchEvent(new KeyboardEvent("keydown", { key: "a" }));
            input?.dispatchEvent(new KeyboardEvent("keydown", { key: "b" }));
            input?.dispatchEvent(new KeyboardEvent("keydown", { key: "c" }));

            // First fires immediately
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "keydown-throttle", expect.objectContaining({ key: "a" }));
        });

        it("should throttle form change events with live-throttle", () => {
            // Scenario: Throttle with different event types (change)
            islandElement.innerHTML = `
                <form live-change="change-throttle">
                    <input name="search" live-throttle="400" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input") as HTMLInputElement;

            // Fire input events rapidly
            input.value = "a";
            input.dispatchEvent(new Event("input", { bubbles: true }));
            input.value = "ab";
            input.dispatchEvent(new Event("input", { bubbles: true }));
            input.value = "abc";
            input.dispatchEvent(new Event("input", { bubbles: true }));

            // First fires immediately
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
        });
    });

    // ---------------------------------------------------------------------------
    // Window Event Tests
    // ---------------------------------------------------------------------------
    // These tests are RED: wireWindowFocusEvents, wireWindowBlurEvents,
    // wireWindowKeydownEvents, wireWindowKeyupEvents do not exist yet.
    // ---------------------------------------------------------------------------

    describe("Window Events", () => {
        it("should handle live-window-focus events fired on the window", () => {
            // Scenario: live-window-focus fires on window focus events
            // Given an element with live-window-focus="focused" inside an island
            // When a focus event fires on the window
            // Then the island receives the "focused" event
            islandElement.innerHTML = '<div live-window-focus="focused">Focus Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            window.dispatchEvent(new Event("focus"));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "focused", {});
        });

        it("should handle live-window-blur events fired on the window", () => {
            // Scenario: live-window-blur fires on window blur events
            islandElement.innerHTML = '<div live-window-blur="blurred">Blur Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            window.dispatchEvent(new Event("blur"));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "blurred", {});
        });

        it("should handle live-window-keydown events with key metadata", () => {
            // Scenario: live-window-keydown fires on window keydown with key metadata
            islandElement.innerHTML = '<div live-window-keydown="shortcut-down">Key Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            window.dispatchEvent(new KeyboardEvent("keydown", { key: "s", ctrlKey: true }));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "shortcut-down", {
                key: "s",
                altKey: false,
                ctrlKey: true,
                shiftKey: false,
                metaKey: false,
            });
        });

        it("should handle live-window-keyup events with key metadata", () => {
            // Scenario: live-window-keyup fires on window keyup with key metadata
            // Given an element with live-window-keyup="shortcut" inside an island
            // When a keyup event fires on the window
            // Then the island receives the "shortcut" event with key data
            islandElement.innerHTML = '<div live-window-keyup="shortcut">Key Up Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            window.dispatchEvent(new KeyboardEvent("keyup", { key: "Enter" }));

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "shortcut", {
                key: "Enter",
                altKey: false,
                ctrlKey: false,
                shiftKey: false,
                metaKey: false,
            });
        });

        it("should filter window keyup events with live-key attribute (matching key fires)", () => {
            // Scenario: live-key filters window key events
            // Given an element with live-window-keyup="up" live-key="ArrowUp" inside an island
            // When ArrowUp is pressed on the window
            // Then the "up" event fires
            islandElement.innerHTML = '<div live-window-keyup="up" live-key="ArrowUp">Arrow Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            window.dispatchEvent(new KeyboardEvent("keyup", { key: "ArrowUp" }));

            expect(mockSendEvent).toHaveBeenCalledTimes(1);
            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "up", expect.objectContaining({ key: "ArrowUp" }));
        });

        it("should filter window keyup events with live-key attribute (non-matching key does not fire)", () => {
            // Scenario: live-key filters window key events
            // When "a" is pressed on the window
            // Then no event fires
            islandElement.innerHTML = '<div live-window-keyup="up" live-key="ArrowUp">Arrow Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            window.dispatchEvent(new KeyboardEvent("keyup", { key: "a" }));

            expect(mockSendEvent).not.toHaveBeenCalled();
        });

        it("should filter window keydown events with live-key attribute", () => {
            islandElement.innerHTML = '<div live-window-keydown="save" live-key="s">Save Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            // Non-matching key should not fire
            window.dispatchEvent(new KeyboardEvent("keydown", { key: "a" }));
            expect(mockSendEvent).not.toHaveBeenCalled();

            // Matching key should fire
            window.dispatchEvent(new KeyboardEvent("keydown", { key: "s" }));
            expect(mockSendEvent).toHaveBeenCalledTimes(1);
            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "save", expect.objectContaining({ key: "s" }));
        });

        it("should add loading class on window events", () => {
            islandElement.innerHTML = '<div live-window-keyup="shortcut">Key Up Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const div = islandElement.querySelector("div") as HTMLElement;
            window.dispatchEvent(new KeyboardEvent("keyup", { key: "Enter" }));

            expect(div.classList.contains("live-window-keyup-loading")).toBe(true);
        });

        it("should remove window event listeners on cleanup", () => {
            // Scenario: Cleanup removes window listeners (no events after cleanup)
            islandElement.innerHTML = '<div live-window-keyup="shortcut">Key Up Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            // Verify it fires before cleanup
            window.dispatchEvent(new KeyboardEvent("keyup", { key: "Enter" }));
            expect(mockSendEvent).toHaveBeenCalledTimes(1);

            // Clean up
            cleanupFn();
            cleanupFn = null;

            // After cleanup, window events should NOT fire
            mockSendEvent.mockClear();
            window.dispatchEvent(new KeyboardEvent("keyup", { key: "Enter" }));
            expect(mockSendEvent).not.toHaveBeenCalled();
        });

        it("should remove window focus/blur listeners on cleanup", () => {
            islandElement.innerHTML = '<div live-window-focus="focused">Focus Listener</div>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            // Verify it fires before cleanup
            window.dispatchEvent(new Event("focus"));
            expect(mockSendEvent).toHaveBeenCalledTimes(1);

            // Clean up
            cleanupFn();
            cleanupFn = null;

            // After cleanup, window events should NOT fire
            mockSendEvent.mockClear();
            window.dispatchEvent(new Event("focus"));
            expect(mockSendEvent).not.toHaveBeenCalled();
        });

        it("should route window events to the declaring island only", () => {
            // Scenario: Multiple islands receive window events independently
            // Given island-A has live-window-keyup="action-a"
            // And island-B has live-window-keyup="action-b"
            // When a keyup event fires on the window
            // Then island-A receives "action-a"
            // And island-B receives "action-b"
            const islandAElement = document.createElement("div");
            islandAElement.setAttribute("data-island-id", "island-a");
            islandAElement.innerHTML = '<div live-window-keyup="action-a">Island A Listener</div>';
            document.body.appendChild(islandAElement);

            const islandBElement = document.createElement("div");
            islandBElement.setAttribute("data-island-id", "island-b");
            islandBElement.innerHTML = '<div live-window-keyup="action-b">Island B Listener</div>';
            document.body.appendChild(islandBElement);

            const cleanupA = wireIslandEvents(islandAElement, "island-a");
            const cleanupB = wireIslandEvents(islandBElement, "island-b");

            window.dispatchEvent(new KeyboardEvent("keyup", { key: "Enter" }));

            // Both islands should receive the event independently
            expect(mockSendEvent).toHaveBeenCalledWith("island-a", "action-a", expect.any(Object));
            expect(mockSendEvent).toHaveBeenCalledWith("island-b", "action-b", expect.any(Object));
            expect(mockSendEvent).toHaveBeenCalledTimes(2);

            // Cleanup
            cleanupA();
            cleanupB();
            document.body.removeChild(islandAElement);
            document.body.removeChild(islandBElement);
        });
    });

    // ---------------------------------------------------------------------------
    // Live-Patch Tests
    // ---------------------------------------------------------------------------
    // These tests are RED: wirePatchEvents does not exist yet.
    // ---------------------------------------------------------------------------

    describe("Live-Patch Navigation", () => {
        let originalPushState: typeof history.pushState;

        beforeEach(() => {
            originalPushState = history.pushState;
            history.pushState = jest.fn();
        });

        afterEach(() => {
            history.pushState = originalPushState;
        });

        it("should prevent default navigation when clicking a live-patch anchor", () => {
            // Scenario: Clicking live-patch updates URL and sends params
            // Given an anchor with live-patch and href="?page=2" inside an island
            // When the user clicks the anchor
            // Then the browser URL is updated
            islandElement.innerHTML = '<a href="?page=2" live-patch>Go to page 2</a>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const anchor = islandElement.querySelector("a") as HTMLAnchorElement;
            const clickEvent = new MouseEvent("click", { bubbles: true, cancelable: true });
            const preventDefaultSpy = jest.spyOn(clickEvent, "preventDefault");
            anchor.dispatchEvent(clickEvent);

            expect(preventDefaultSpy).toHaveBeenCalled();
        });

        it("should update URL via history.pushState when clicking a live-patch anchor", () => {
            // Scenario: Clicking live-patch updates URL and sends params
            // Then the browser URL is updated to include page=2
            islandElement.innerHTML = '<a href="?page=2" live-patch>Go to page 2</a>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const anchor = islandElement.querySelector("a");
            anchor?.click();

            expect(history.pushState).toHaveBeenCalled();
            const callArgs = (history.pushState as jest.Mock).mock.calls[0];
            // The new URL should contain "page=2"
            expect(callArgs[2]).toContain("page=2");
        });

        it("should send a params event with URL search params when clicking a live-patch anchor", () => {
            // Scenario: Clicking live-patch updates URL and sends params
            // And a params event is sent to the server with page=2
            islandElement.innerHTML = '<a href="?page=2" live-patch>Go to page 2</a>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const anchor = islandElement.querySelector("a");
            anchor?.click();

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "params", expect.objectContaining({ page: "2" }));
        });

        it("should send a params event with multiple search params", () => {
            islandElement.innerHTML = '<a href="?page=3&sort=asc" live-patch>Go to page 3</a>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const anchor = islandElement.querySelector("a");
            anchor?.click();

            expect(mockSendEvent).toHaveBeenCalledWith(islandId, "params", expect.objectContaining({
                page: "3",
                sort: "asc",
            }));
        });

        it("should handle live-patch anchor without href gracefully (no error)", () => {
            // Scenario: Handles elements without href gracefully
            islandElement.innerHTML = '<a live-patch>No href</a>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const anchor = islandElement.querySelector("a");

            // Should not throw
            expect(() => anchor?.click()).not.toThrow();
        });

        it("should not fire events after cleanup for live-patch anchors", () => {
            islandElement.innerHTML = '<a href="?page=2" live-patch>Go to page 2</a>';
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const anchor = islandElement.querySelector("a");

            // Clean up
            cleanupFn();
            cleanupFn = null;

            anchor?.click();

            // No event and no pushState after cleanup
            expect(mockSendEvent).not.toHaveBeenCalled();
            expect(history.pushState).not.toHaveBeenCalled();
        });
    });

    // ---------------------------------------------------------------------------
    // Upload Tests (RED)
    // ---------------------------------------------------------------------------
    // These tests reference upload progress and file-input validation behaviour
    // that does NOT yet exist in wireIslandEvents.
    //
    // Scenario: Upload progress event dispatched during XHR upload
    //   Given a form with live-submit and a file input inside an island
    //   When the form is submitted with a file attached
    //   Then a "live-upload-progress" CustomEvent is dispatched on the form
    //   And the event detail contains { loaded, total } reflecting XHR progress
    //
    // Scenario: File input change triggers validation event
    //   Given a file input with live-upload="<config-name>" inside an island
    //   When the user selects a file using the file input
    //   Then a "validate" event is sent to the server
    //   And the event payload contains the upload metadata (name, size, type)
    // ---------------------------------------------------------------------------

    describe("Upload Events (RED)", () => {
        let xhrMock: {
            open: jest.Mock;
            send: jest.Mock;
            addEventListener: jest.Mock;
            upload: {
                addEventListener: jest.Mock;
            };
        };

        beforeEach(() => {
            // Build a minimal XHR mock that captures upload.onprogress listener.
            xhrMock = {
                open: jest.fn(),
                send: jest.fn(),
                addEventListener: jest.fn(),
                upload: {
                    addEventListener: jest.fn(),
                },
            };

            // Replace global XMLHttpRequest with the mock.
            (global as any).XMLHttpRequest = jest.fn(() => xhrMock);
        });

        afterEach(() => {
            // Restore the real XMLHttpRequest after each test.
            delete (global as any).XMLHttpRequest;
        });

        it("should dispatch a live-upload-progress event on the form during XHR upload", () => {
            // Scenario: Upload progress event dispatched during XHR upload
            // Given a form with live-submit and a file input
            islandElement.innerHTML = `
                <form live-submit="submit-event" id="upload-form">
                    <input name="avatar" type="file" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const form = islandElement.querySelector("form") as HTMLFormElement;

            // Listen for the progress custom event on the form.
            const progressEvents: CustomEvent[] = [];
            form.addEventListener("live-upload-progress", (e: Event) => {
                progressEvents.push(e as CustomEvent);
            });

            // Submit the form.
            form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

            // The XHR should have been created and upload.addEventListener called with "progress".
            expect(xhrMock.upload.addEventListener).toHaveBeenCalledWith(
                "progress",
                expect.any(Function)
            );

            // Retrieve the progress handler and invoke it with synthetic progress data.
            const [, progressHandler] = (xhrMock.upload.addEventListener as jest.Mock).mock.calls.find(
                ([event]: [string]) => event === "progress"
            ) ?? [];

            if (!progressHandler) {
                throw new Error("progress handler was not registered on xhr.upload");
            }

            const progressEvent = { loaded: 512, total: 1024 };
            progressHandler(progressEvent);

            // The form should have dispatched a live-upload-progress CustomEvent.
            expect(progressEvents).toHaveLength(1);
            expect(progressEvents[0].detail).toMatchObject({ loaded: 512, total: 1024 });
        });

        it("should send a validate event when a file input changes", () => {
            // Scenario: File input change triggers validation event
            // Given a file input with live-upload="avatar" inside an island
            islandElement.innerHTML = `
                <input type="file" live-upload="avatar" name="avatar" />
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const input = islandElement.querySelector("input[type='file']") as HTMLInputElement;

            // Simulate the user selecting a file by defining the files property.
            const mockFile = new File(["hello"], "photo.png", {
                type: "image/png",
                lastModified: 1700000000000,
            });

            Object.defineProperty(input, "files", {
                value: [mockFile],
                writable: false,
                configurable: true,
            });

            // Dispatch the change event on the file input.
            input.dispatchEvent(new Event("change", { bubbles: true }));

            // The island should have sent a "validate" event containing upload metadata.
            expect(mockSendEvent).toHaveBeenCalledWith(
                islandId,
                "validate",
                expect.objectContaining({
                    uploads: expect.objectContaining({
                        avatar: expect.arrayContaining([
                            expect.objectContaining({
                                name: "photo.png",
                                size: mockFile.size,
                                type: "image/png",
                            }),
                        ]),
                    }),
                })
            );
        });

        it("should not dispatch live-upload-progress when form has no file input", () => {
            // Regression guard: plain text-only forms should not set up an XHR progress listener.
            islandElement.innerHTML = `
                <form live-submit="submit-event">
                    <input name="username" value="john" />
                </form>
            `;
            cleanupFn = wireIslandEvents(islandElement, islandId);

            const form = islandElement.querySelector("form") as HTMLFormElement;
            form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

            // XHR should not have been created for a form without files.
            expect(xhrMock.upload.addEventListener).not.toHaveBeenCalled();
        });
    });
});
