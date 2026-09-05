import { Download } from '../../wailsjs/wailsjs/go/main/App';

export const useDownloadEngine = () => {
    const startDownload = async (id: string, url: string, path: string): Promise<void> => {
        await Download(id, url, path);
    };

    return { startDownload };
};
