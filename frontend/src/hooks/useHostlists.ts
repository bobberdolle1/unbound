import { useState, useCallback } from 'react';
import { backendService } from '../services/backend';
import { windowService } from '../services/window';

export function useHostlists() {
  const [hostlists, setHostlists] = useState<string[]>([]);
  const [selectedList, setSelectedList] = useState<string>('');
  const [hostlistContent, setHostlistContent] = useState<string>('');
  const [isSavingHostlist, setIsSavingHostlist] = useState<boolean>(false);

  const handleOpenHostlistEditor = useCallback(async () => {
    try {
      const lists = await backendService.getBypassLists();
      setHostlists(lists);
      if (lists.length > 0) {
        setSelectedList(lists[0]);
        const content = await backendService.readBypassList(lists[0]);
        setHostlistContent(content);
      }
    } catch (err) {
      console.error('Failed to load hostlists:', err);
    }
  }, []);

  const handleSelectHostlist = useCallback(async (name: string) => {
    setSelectedList(name);
    try {
      const content = await backendService.readBypassList(name);
      setHostlistContent(content);
    } catch (err) {
      console.error('Failed to read hostlist:', err);
    }
  }, []);

  const handleSaveHostlist = useCallback(async () => {
    setIsSavingHostlist(true);
    try {
      await backendService.saveBypassList(selectedList, hostlistContent);
      windowService.showNotification('Успех', `Список ${selectedList} сохранен.`);
    } catch (err) {
      windowService.showNotification('Ошибка', `Не удалось сохранить список: ${err}`);
    } finally {
      setIsSavingHostlist(false);
    }
  }, [selectedList, hostlistContent]);

  return {
    state: {
      hostlists,
      selectedList,
      hostlistContent,
      isSavingHostlist,
    },
    actions: {
      setHostlistContent,
      handleOpenHostlistEditor,
      handleSelectHostlist,
      handleSaveHostlist,
    },
  };
}
