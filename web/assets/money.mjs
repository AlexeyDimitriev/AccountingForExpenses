// parseAmount переводит рубли в копейки без арифметики с плавающей точкой.
export function parseAmount(value) {
  const normalized = value.trim().replace(',', '.');
  if (!/^\d+(?:\.\d{1,2})?$/.test(normalized)) throw new Error('Укажите положительную сумму с точностью до копеек.');
  const [rubles, fraction = ''] = normalized.split('.');
  const kopecks = BigInt(rubles) * 100n + BigInt(fraction.padEnd(2, '0'));
  if (kopecks <= 0n || kopecks > BigInt(Number.MAX_SAFE_INTEGER)) throw new Error('Сумма должна быть от 0,01 до 90 071 992 547 409,91 ₽.');
  return Number(kopecks);
}

// amountInput возвращает точную десятичную запись суммы для формы.
export function amountInput(kopecks) {
  if (!Number.isSafeInteger(kopecks) || kopecks < 0) throw new Error('Сумма слишком велика для точного отображения в браузере.');
  const value = BigInt(kopecks);
  return `${value / 100n}.${String(value % 100n).padStart(2, '0')}`;
}

// formatAmount форматирует целое число копеек как сумму в рублях.
export function formatAmount(kopecks) {
  const [rubles, fraction] = amountInput(kopecks).split('.');
  return `${rubles.replace(/\B(?=(\d{3})+(?!\d))/g, '\u00a0')},${fraction}\u00a0₽`;
}
