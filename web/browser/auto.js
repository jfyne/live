(()=>{var _=Object.defineProperty;var x=Object.getOwnPropertySymbols;var j=Object.prototype.hasOwnProperty,J=Object.prototype.propertyIsEnumerable;var M=(i,t,e)=>t in i?_(i,t,{enumerable:!0,configurable:!0,writable:!0,value:e}):i[t]=e,h=(i,t)=>{for(var e in t||(t={}))j.call(t,e)&&M(i,e,t[e]);if(x)for(var e of x(t))J.call(t,e)&&M(i,e,t[e]);return i};var y=class{static hook(t){return t.getAttribute===void 0?null:t.getAttribute("live-hook")}};var X="live:mounted",Q="live:beforeupdate",V="live:updated",Y="live:beforedestroy",Z="live:destroyed",tt="live:disconnected",et="live:reconnected",T="live-connected",A="live-disconnected",st="live-error",k=class{constructor(t,e,s){this.typ=t,this.data=e,s!==void 0?this.id=s:this.id=0}static GetID(){return this.sequence++}serialize(){return JSON.stringify({t:this.typ,i:this.id,d:this.data})}static fromMessage(t){let e=JSON.parse(t);return new k(e.t,e.d,e.i)}},o=k;o.sequence=1;var a=class{constructor(){}static init(t,e){this.hooks=t,this.dom=e,this.eventHandlers={}}static handleEvent(t){t.typ in this.eventHandlers&&this.eventHandlers[t.typ].map(e=>{e(t.data)})}static mounted(t){let e=new CustomEvent(X,{}),s=this.getElementHooks(t);s!==null&&this.callHook(e,t,s.mounted)}static beforeUpdate(t,e){let s=new CustomEvent(Q,{}),n=this.getElementHooks(t);n!==null&&this.callHook(s,t,n.beforeUpdate),this.dom!==void 0&&this.dom.onBeforeElUpdated!==void 0&&this.dom.onBeforeElUpdated(t,e)}static updated(t){let e=new CustomEvent(V,{}),s=this.getElementHooks(t);s!==null&&this.callHook(e,t,s.updated)}static beforeDestroy(t){let e=new CustomEvent(Y,{}),s=this.getElementHooks(t);s!==null&&this.callHook(e,t,s.beforeDestroy)}static destroyed(t){let e=new CustomEvent(Z,{}),s=this.getElementHooks(t);s!==null&&this.callHook(e,t,s.destroyed)}static disconnected(){let t=new CustomEvent(tt,{});document.querySelectorAll("[live-hook]").forEach(e=>{let s=this.getElementHooks(e);s!==null&&this.callHook(t,e,s.disconnected)}),document.body.classList.add(A),document.body.classList.remove(T)}static reconnected(){let t=new CustomEvent(et,{});document.querySelectorAll("[live-hook]").forEach(e=>{let s=this.getElementHooks(e);s!==null&&this.callHook(t,e,s.reconnected)}),document.body.classList.remove(A),document.body.classList.add(T)}static error(){document.body.classList.add(st)}static getElementHooks(t){let e=y.hook(t);return e===null?e:this.hooks[e]}static callHook(t,e,s){if(s===void 0)return;let n=d=>{c.send(d)},r=(d,m)=>{d in this.eventHandlers||(this.eventHandlers[d]=[]),this.eventHandlers[d].push(m)};s.bind({el:e,pushEvent:n,handleEvent:r})(),e.dispatchEvent(t)}};var l=class{static dehydrate(){document.querySelectorAll("form").forEach(e=>{if(e.id===""){console.error("form does not have an ID. DOM updates may be affected",e);return}this.formState[e.id]=[],new FormData(e).forEach((s,n)=>{let r={name:n,value:s,focus:e.querySelector(`[name="${n}"]`)==document.activeElement};this.formState[e.id].push(r)})})}static hydrate(){Object.keys(this.formState).map(t=>{let e=document.querySelector(`#${t}`);if(e===null){delete this.formState[t];return}this.formState[t].map(n=>{let r=e.querySelector(`[name="${n.name}"]`);if(r!==null)switch(r.type){case"file":break;case"checkbox":n.value==="on"&&(r.checked=!0);break;default:r.value=n.value,n.focus===!0&&r.focus();break}})})}static serialize(t){let e={};return new FormData(t).forEach((n,r)=>{switch(!0){case n instanceof File:let d=n,m={name:d.name,type:d.type,size:d.size,lastModified:d.lastModified};Reflect.has(e,this.upKey)||(e[this.upKey]={}),Reflect.has(e[this.upKey],r)||(e[this.upKey][r]=[]),e[this.upKey][r].push(m);break;default:if(!Reflect.has(e,r)){e[r]=n;return}Array.isArray(e[r])||(e[r]=[e[r]]),e[r].push(n)}}),e}static hasFiles(t){let e=new FormData(t),s=!1;return e.forEach(n=>{n instanceof File&&(s=!0)}),s}};l.upKey="uploads",l.formState={};var p=class{static handle(t){l.dehydrate(),t.data.map(p.applyPatch),l.hydrate()}static applyPatch(t){let e=document.querySelector(`*[${t.Anchor}]`);if(e===null)return;let s=p.html2Node(t.HTML);switch(t.Action){case 0:return;case 1:t.HTML===""?a.beforeDestroy(e):a.beforeUpdate(e,s),e.outerHTML=t.HTML,t.HTML===""?a.destroyed(e):a.updated(e);break;case 2:a.beforeUpdate(e,s),e.append(s),a.updated(e);break;case 3:a.beforeUpdate(e,s),e.prepend(s),a.updated(e);break}}static html2Node(t){let e=document.createElement("template");return t=t.trim(),e.innerHTML=t,e.content.firstChild===null?document.createTextNode(t):e.content.firstChild}};function E(i){let t={};if(new URLSearchParams(window.location.search).forEach((n,r)=>{t[r]=n}),i===void 0||!i.hasAttributes())return t;let s=i.attributes;for(let n=0;n<s.length;n++)!s[n].name.startsWith("live-value-")||(t[s[n].name.split("live-value-")[1]]=s[n].value);return t}function w(i){let t=new URL(i,location.origin),e=new URLSearchParams(t.search),s={};return e.forEach((n,r)=>{s[r]=n}),s}function b(i,t){if(window.history.pushState({},"",i),t===void 0)c.send(new o("params",h({},w(i))));else{let e=E(t);c.sendAndTrack(new o("params",h(h({},e),w(i)),o.GetID()),t)}}var u=class{constructor(t,e){this.event=t;this.attribute=e;this.limiter=new L}isWired(t){return t.hasAttribute(`${this.attribute}-wired`)?!0:(t.setAttribute(`${this.attribute}-wired`,""),!1)}attach(){document.querySelectorAll(`*[${this.attribute}]`).forEach(t=>{if(this.isWired(t)==!0)return;let e=E(t);t.addEventListener(this.event,s=>{this.limiter.hasDebounce(t)?this.limiter.debounce(t,s,this.handler(t,e)):this.handler(t,e)(s)}),t.addEventListener("ack",s=>{t.classList.remove(`${this.attribute}-loading`)})})}windowAttach(){document.querySelectorAll(`*[${this.attribute}]`).forEach(t=>{if(this.isWired(t)===!0)return;let e=E(t);window.addEventListener(this.event,this.handler(t,e)),window.addEventListener("ack",s=>{t.classList.remove(`${this.attribute}-loading`)})})}handler(t,e){return s=>{let n=t==null?void 0:t.getAttribute(this.attribute);n!==null&&(t.classList.add(`${this.attribute}-loading`),c.sendAndTrack(new o(n,e,o.GetID()),t))}}},f=class extends u{handler(t,e){return s=>{let n=s,r=t==null?void 0:t.getAttribute(this.attribute);if(r===null)return;let d=t.getAttribute("live-key");if(d!==null&&n.key!==d)return;t.classList.add(`${this.attribute}-loading`);let m={key:n.key,altKey:n.altKey,ctrlKey:n.ctrlKey,shiftKey:n.shiftKey,metaKey:n.metaKey};c.sendAndTrack(new o(r,h(h({},e),m),o.GetID()),t)}}},L=class{constructor(){this.debounceAttr="live-debounce"}hasDebounce(t){return t.hasAttribute(this.debounceAttr)}debounce(t,e,s){if(clearTimeout(this.debounceEvent),!this.hasDebounce(t)){s(e);return}let n=t.getAttribute(this.debounceAttr);if(n===null){s(e);return}if(n==="blur"){this.debounceEvent=s,t.addEventListener("blur",()=>{this.debounceEvent()});return}this.debounceEvent=setTimeout(()=>{s(e)},parseInt(n))}},D=class extends u{constructor(){super("click","live-click")}},S=class extends u{constructor(){super("contextmenu","live-contextmenu")}},$=class extends u{constructor(){super("mousedown","live-mousedown")}},P=class extends u{constructor(){super("mouseup","live-mouseup")}},F=class extends u{constructor(){super("focus","live-focus")}},K=class extends u{constructor(){super("blur","live-blur")}},C=class extends u{constructor(){super("focus","live-window-focus")}attach(){this.windowAttach()}},U=class extends u{constructor(){super("blur","live-window-blur")}attach(){this.windowAttach()}},q=class extends f{constructor(){super("keydown","live-keydown")}},W=class extends f{constructor(){super("keyup","live-keyup")}},I=class extends f{constructor(){super("keydown","live-window-keydown")}attach(){this.windowAttach()}},R=class extends f{constructor(){super("keyup","live-window-keyup")}attach(){this.windowAttach()}},N=class{constructor(){this.attribute="live-change";this.limiter=new L}isWired(t){return t.hasAttribute(`${this.attribute}-wired`)?!0:(t.setAttribute(`${this.attribute}-wired`,""),!1)}attach(){let t=[];document.querySelectorAll(`form[${this.attribute}]`).forEach(e=>{e.addEventListener("ack",s=>{e.classList.remove(`${this.attribute}-loading`)}),t.push(e),e.querySelectorAll("input,select,textarea").forEach(s=>{this.addEvent(e,s)})}),t.forEach(e=>{document.querySelectorAll(`[form=${e.getAttribute("id")}]`).forEach(s=>{this.addEvent(e,s)})})}addEvent(t,e){this.isWired(e)||e.addEventListener("input",s=>{this.limiter.hasDebounce(e)?this.limiter.debounce(e,s,()=>{this.handler(t)}):this.handler(t)})}handler(t){let e=t==null?void 0:t.getAttribute(this.attribute);if(e===null)return;let s=l.serialize(t);t.classList.add(`${this.attribute}-loading`),c.sendAndTrack(new o(e,s,o.GetID()),t)}},O=class extends u{constructor(){super("submit","live-submit")}handler(t,e){return s=>{if(s.preventDefault&&s.preventDefault(),l.hasFiles(t)===!0){let r=new XMLHttpRequest;r.open("POST",""),r.addEventListener("load",()=>{this.sendEvent(t,e)}),r.send(new FormData(t))}else this.sendEvent(t,e);return!1}}sendEvent(t,e){let s=t==null?void 0:t.getAttribute(this.attribute);if(s===null)return;var n=h({},e);let r=l.serialize(t);Object.keys(r).map(d=>{n[d]=r[d]}),t.classList.add(`${this.attribute}-loading`),c.sendAndTrack(new o(s,n,o.GetID()),t)}},B=class extends u{constructor(){super("","live-hook")}attach(){document.querySelectorAll(`[${this.attribute}]`).forEach(t=>{this.isWired(t)!=!0&&a.mounted(t)})}},G=class extends u{constructor(){super("click","live-patch")}handler(t,e){return s=>{s.preventDefault&&s.preventDefault();let n=t.getAttribute("href");if(n!==null)return b(n,t),!1}}},v=class{static init(){this.clicks=new D,this.contextmenu=new S,this.mousedown=new $,this.mouseup=new P,this.focus=new F,this.blur=new K,this.windowFocus=new C,this.windowBlur=new U,this.keydown=new q,this.keyup=new W,this.windowKeydown=new I,this.windowKeyup=new R,this.change=new N,this.submit=new O,this.hook=new B,this.patch=new G,this.handleBrowserNav()}static rewire(){this.clicks.attach(),this.contextmenu.attach(),this.mousedown.attach(),this.mouseup.attach(),this.focus.attach(),this.blur.attach(),this.windowFocus.attach(),this.windowBlur.attach(),this.keydown.attach(),this.keyup.attach(),this.windowKeyup.attach(),this.windowKeydown.attach(),this.change.attach(),this.submit.attach(),this.hook.attach(),this.patch.attach()}static handleBrowserNav(){window.onpopstate=function(t){c.send(new o("params",w(document.location.search),o.GetID()))}}};var z="_psid",g=class{constructor(){}static getID(){if(this.id)return this.id;let e=`; ${document.cookie}`.split(`; ${z}=`);if(e&&e.length===2){let s=e.pop();return s?s.split(";").shift():""}return""}static setCookie(){var t=new Date;t.setTime(t.getTime()+60*1e3),document.cookie=`${z}=${this.id}; expires=${t.toUTCString()}; path=/`}static dial(){this.trackedEvents={},this.id=this.getID(),this.setCookie(),console.debug("Socket.dial called",this.id),this.conn=new WebSocket(`${location.protocol==="https:"?"wss":"ws"}://${location.host}${location.pathname}${location.search}${location.hash}`),this.conn.addEventListener("close",t=>{this.ready=!1,console.warn(`WebSocket Disconnected code: ${t.code}, reason: ${t.reason}`),t.code!==1001&&(this.disconnectNotified===!1&&(a.disconnected(),this.disconnectNotified=!0),setTimeout(()=>{g.dial()},1e3))}),this.conn.addEventListener("open",t=>{a.reconnected(),this.disconnectNotified=!1,this.ready=!0}),this.conn.addEventListener("message",t=>{if(typeof t.data!="string"){console.error("unexpected message type",typeof t.data);return}let e=o.fromMessage(t.data);switch(e.typ){case"patch":p.handle(e),v.rewire();break;case"params":b(`${window.location.pathname}?${e.data}`);break;case"redirect":window.location.replace(e.data);break;case"ack":this.ack(e);break;case"err":a.error();default:a.handleEvent(e)}})}static sendAndTrack(t,e){if(this.ready===!1){console.warn("connection not ready for send of event",t);return}this.trackedEvents[t.id]={ev:t,el:e},this.conn.send(t.serialize())}static send(t){if(this.ready===!1){console.warn("connection not ready for send of event",t);return}this.conn.send(t.serialize())}static ack(t){t.id in this.trackedEvents&&(this.trackedEvents[t.id].el.dispatchEvent(new Event("ack")),delete this.trackedEvents[t.id])}},c=g;c.ready=!1,c.disconnectNotified=!1;var H=class{constructor(t,e){this.hooks=t;this.dom=e}init(){document.querySelector("[live-rendered]")!==null&&(a.init(this.hooks,this.dom),c.dial(),v.init(),v.rewire())}send(t,e,s){let n=new o(t,e,s);c.send(n)}};document.addEventListener("DOMContentLoaded",i=>{window.Live!==void 0&&console.error("window.Live already defined");let t=window.Hooks||{};window.Live=new H(t),window.Live.init()});})();
//# sourceMappingURL=auto.js.map
