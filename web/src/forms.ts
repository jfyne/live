/**
 * A value of an existing input in a form.
 */
interface inputState {
    name: string;
    focus: boolean;
    value: any;
}

/**
 * Form helper class.
 */
export class Forms {
    private static formState: { [id: string]: inputState[] } = {};

    /**
     * When we are patching the DOM we need to save the state
     * of any forms so that we don't lose input values or
     * focus
     */
    static dehydrate() {
        const forms = document.querySelectorAll("form");
        forms.forEach((f) => {
            if (f.id === "") {
                console.error(
                    "form does not have an ID. DOM updates may be affected",
                    f
                );
                return;
            }

            this.formState[f.id] = [];
            new FormData(f).forEach((value: any, name: string) => {
                const i = {
                    name: name,
                    value: value,
                    focus:
                        f.querySelector(`[name="${name}"]`) ==
                        document.activeElement,
                };
                this.formState[f.id].push(i);
            });
        });
    }

    /**
     * This sets the form backup to its original state.
     */
    static hydrate() {
        Object.keys(this.formState).map((formID) => {
            const form = document.querySelector(`#${formID}`);
            if (form === null) {
                delete this.formState[formID];
                return;
            }

            const state = this.formState[formID];
            state.map((i) => {
                const input = form.querySelector(
                    `[name="${i.name}"]`
                ) as HTMLInputElement;
                if (input === null) {
                    return;
                }
                switch (input.type) {
                    case "checkbox":
                        if (i.value === "on") {
                            input.checked = true;
                        }
                    default:
                        input.value = i.value;
                        if (i.focus === true) {
                            input.focus();
                        }
                }
            });
        });
    }
}
