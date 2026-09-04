export const useDownloadEngine = () => {
    const startDownload = async (url: string): Promise<void> => {
        console.log('Backend Download:', url);
    };

    return { startDownload };
};
