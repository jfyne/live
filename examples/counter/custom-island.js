// Custom island mounting for the counter example
// This sends the island props to the server when subscribing

(function() {
    // Wait for the Live library to load
    if (typeof customElements === 'undefined') {
        console.error('Custom Elements not supported');
        return;
    }

    // Global WebSocket connection shared by all islands
    window.liveWS = null;
    const pendingIslands = [];

    // Override the LiveIsland class to send props with subscribe
    class CounterLiveIsland extends HTMLElement {
        constructor() {
            super();
            this.mounted = false;
            this.islandId = null;
            this.props = null;
        }

        connectedCallback() {
            const type = this.getAttribute('type');
            const id = this.getAttribute('id');

            if (!type || !id) {
                console.error('LiveIsland missing type or id attribute');
                return;
            }

            this.islandId = id;

            // Extract all data-* attributes as props
            const props = { type, id };
            Array.from(this.attributes).forEach(attr => {
                if (attr.name.startsWith('data-')) {
                    const key = attr.name.substring(5).replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
                    props[key] = attr.value;
                }
            });

            this.props = props;

            // Add to pending islands queue
            pendingIslands.push(this);
            console.log('Island connected:', id, 'Pending:', pendingIslands.length);

            // Try to mount if WebSocket is ready
            if (window.liveWS && window.liveWS.readyState === WebSocket.OPEN) {
                this.mountOnServer();
            }

            // Set up event listeners for buttons
            this.addEventListener('click', (e) => {
                const target = e.target;
                if (target.hasAttribute('live-click')) {
                    const eventType = target.getAttribute('live-click');
                    this.sendEvent(eventType, {});
                }
            });

            this.mounted = true;
        }

        mountOnServer() {
            if (!window.liveWS || window.liveWS.readyState !== WebSocket.OPEN) {
                console.warn('Cannot mount island, WebSocket not ready:', this.islandId);
                return false;
            }

            const message = {
                t: 'subscribe',
                island: this.islandId,
                d: this.props
            };
            window.liveWS.send(JSON.stringify(message));
            console.log('Sent subscribe message:', this.islandId, this.props);
            return true;
        }

        sendEvent(eventType, data) {
            if (window.liveWS && window.liveWS.readyState === WebSocket.OPEN) {
                const message = {
                    t: eventType,
                    island: this.islandId,
                    d: data
                };
                window.liveWS.send(JSON.stringify(message));
                console.log('Sent event:', eventType, 'for island:', this.islandId);
            } else {
                console.warn('Cannot send event, WebSocket not ready');
            }
        }

        disconnectedCallback() {
            this.mounted = false;
            // Remove from pending islands if still there
            const index = pendingIslands.indexOf(this);
            if (index > -1) {
                pendingIslands.splice(index, 1);
            }
        }
    }

    // Register the custom element
    if (!customElements.get('live-island')) {
        customElements.define('live-island', CounterLiveIsland);
        console.log('CounterLiveIsland custom element registered');
    }

    // Set up WebSocket connection
    function connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        console.log('Connecting to WebSocket:', wsUrl);
        const ws = new WebSocket(wsUrl);
        window.liveWS = ws;

        ws.onopen = () => {
            console.log('WebSocket connected, mounting', pendingIslands.length, 'pending islands');

            // Mount all pending islands
            pendingIslands.forEach(island => {
                island.mountOnServer();
            });
            // Don't clear the array - islands stay registered
        };

        ws.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                console.log('WebSocket received:', message.t, 'for island:', message.island);

                // Handle patch messages
                if (message.t === 'patch' && message.island) {
                    const island = document.getElementById(message.island);
                    if (island && message.d && message.d.html) {
                        island.innerHTML = message.d.html;
                        console.log('Updated island:', message.island);
                    }
                }
            } catch (e) {
                console.error('Error parsing message:', e, event.data);
            }
        };

        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        ws.onclose = () => {
            console.log('WebSocket closed, reconnecting in 1s...');
            window.liveWS = null;
            setTimeout(connectWebSocket, 1000);
        };
    }

    // Connect WebSocket when page loads
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', connectWebSocket);
    } else {
        connectWebSocket();
    }
})();
