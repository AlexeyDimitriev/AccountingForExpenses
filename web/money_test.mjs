import test from 'node:test';
import assert from 'node:assert/strict';
import { parseAmount, amountInput, formatAmount } from './assets/money.mjs';

// Проверяем точный перевод рублей, включая значения, неточные в формате float.
test('точный перевод рублей в копейки', () => {
  for (const [text, expected] of [['0,01', 1], ['0.29', 29], ['1.1', 110], [' 1234,56 ', 123456], ['10', 1000], ['90071992547409.91', Number.MAX_SAFE_INTEGER]]) {
    assert.equal(parseAmount(text), expected);
    assert.equal(parseAmount(amountInput(expected)), expected);
  }
});

// Проверяем отказ вместо округления или переполнения суммы.
test('отклонение недопустимых сумм', () => {
  for (const value of ['', '0', '-1', '1.001', '1e3', 'Infinity', 'NaN', '1,2.3', '90071992547409.92']) {
    assert.throws(() => parseAmount(value));
  }
  for (const value of [Number.MAX_SAFE_INTEGER + 1, -1, 1.5, NaN, Infinity]) assert.throws(() => formatAmount(value));
});

// Проверяем отображение нулевого итога и больших сумм без потери копеек.
test('форматирование рублей', () => {
  assert.equal(formatAmount(0), '0,00\u00a0₽');
  assert.equal(formatAmount(123456), '1\u00a0234,56\u00a0₽');
  assert.equal(formatAmount(Number.MAX_SAFE_INTEGER), '90\u00a0071\u00a0992\u00a0547\u00a0409,91\u00a0₽');
});
