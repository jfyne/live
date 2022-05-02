import { Socket } from "./socket";
import { LiveEvent, EventUpload } from "./event";

const bytesPerChunk = 1024 * 20;

interface uploadPrimer {
    name: string;
    type: string;
    size: number;
}

export class Upload {
    fieldName: string;
    file: File;
    name: string;
    type: string;
    size: number;

    constructor(fieldName: string, file: File) {
        this.fieldName = fieldName;
        this.file = file;
        this.name = file.name;
        this.type = file.type;
        this.size = file.size;
    }

    async begin() {
        this.sendPrimer({
            name: this.name,
            type: this.type,
            size: this.size,
        });

        const blob = this.file as Blob;
        let start = 0;
        let end = bytesPerChunk;

        while (start < blob.size) {
            const slice = blob.slice(start, end);
            Socket.sendFile(slice);
            start = end;
            end = start + bytesPerChunk;
        }
    }

    private sendPrimer(primer: uploadPrimer) {
        Socket.send(new LiveEvent(EventUpload, primer));
    }
}
