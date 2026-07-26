export const formatLog = (log: string) => {
  let formatted = log;
  if (formatted.includes('[STDOUT]')) formatted = formatted.replace('[STDOUT]', '').trim();
  if (formatted.includes('[STDERR]')) formatted = formatted.replace('[STDERR]', '').trim();
  return formatted;
};
