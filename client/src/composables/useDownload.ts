import { CancelDownload, QueueDownload } from '../../wailsjs/wailsjs/go/main/App';

export const useDownloadEngine = () => {
    const queueDownload = async (id: string, url: string, path: string): Promise<void> => {
        await QueueDownload(id, url, path);
    };

    const cancelDownload = (id: string) => CancelDownload(id);

    return { queueDownload, cancelDownload };
};
