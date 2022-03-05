/**
 * A value of an existing input in a form.
 */
interface inputState {
    name: string;
    focus: boolean;
    value: any;
}

/**
 * A value of a file input for validation.
 */
interface fileInput {
    name: string;
    lastModified: number;
    size: number;
    type: string;
}

/**
 * Form helper class.
 */
export class Forms {
    private static upKey = "uploads";

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
                    case "file":
                        break;
                    case "checkbox":
                        if (i.value === "on") {
                            input.checked = true;
                        }
                        break;
                    default:
                        input.value = i.value;
                        if (i.focus === true) {
                            input.focus();
                        }
                        break;
                }
            });
        });
    }

    /**
     * serialize form to values.
     */
    static serialize(form: HTMLFormElement): { [key: string]: string | number | fileInput } {
        const values: { [key: string]: any } = {};
        const formData = new FormData(form);
        formData.forEach((value, key) => {
            switch (true) {
                case value instanceof File:
                    const file = value as File;
                    const fi = {
                        name: file.name,
                        type: file.type,
                        size: file.size,
                        lastModified: file.lastModified,
                    }
                    if (!Reflect.has(values, this.upKey)) {
                        values[this.upKey] = {};
                    }
                    if (!Reflect.has(values[this.upKey], key)) {
                        values[this.upKey][key] = [];
                    }
                    values[this.upKey][key].push(fi);
                    break;
                default:
                    // If the key doesn exist set it.
                    if (!Reflect.has(values, key)) {
                        values[key] = value;
                        return;
                    }
                    // If it already exists that means this needs to become
                    // and array.
                    if (!Array.isArray(values[key])) {
                        values[key] = [values[key]];
                    }
                    // Push the new value onto the array.
                    values[key].push(value);
            }
        });
        return values;
    }

    /**
     * does a form have files.
     */
    static hasFiles(form: HTMLFormElement): boolean {
        const formData = new FormData(form);
        let hasFiles = false;
        formData.forEach((value) => {
            if(value instanceof File) {
                hasFiles = true;
            }
        });
        return hasFiles;
    }
}
