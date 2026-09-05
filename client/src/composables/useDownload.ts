import { Download } from '../../wailsjs/wailsjs/go/main/App';

export const useDownloadEngine = () => {
    const startDownload = async (url: string, path: string): Promise<void> => {
        await Download(url, path);
    };

    return { startDownload };
};
