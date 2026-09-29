import { parseAmount, amountInput, formatAmount } from './money.mjs';

const state = { categories: [], expenses: [], total: null, loaded: false, busy: false, expenseID: null, categoryID: null };
// element находит элемент интерфейса по идентификатору.
const element = id => document.getElementById(id);

// showNotice показывает результат действия или ошибку загрузки.
function showNotice(message = '', error = false) {
  const notice = element('notice');
  notice.textContent = message;
  notice.hidden = !message;
  notice.classList.toggle('error', error);
}

// setBusy блокирует повторные действия до завершения запроса.
function setBusy(busy) {
  state.busy = busy;
  document.querySelectorAll('button, input, select').forEach(control => { control.disabled = busy; });
  element('loading').hidden = !busy;
  document.querySelector('main').setAttribute('aria-busy', String(busy));
  element('add-expense').disabled = busy || !state.loaded || state.categories.length === 0;
  element('add-category').disabled = busy || !state.loaded;
  document.querySelectorAll('[data-action]').forEach(button => { button.disabled = busy || !state.loaded; });
}

// perform выполняет действие с общей обработкой ошибок и состояния загрузки.
async function perform(action, errorID) {
  if (state.busy) return;
  showNotice();
  if (errorID) element(errorID).hidden = true;
  setBusy(true);
  try { await action(); }
  catch (error) {
    const target = errorID && element(errorID);
    if (target && target.closest('dialog').open) {
      target.textContent = error.message;
      target.hidden = false;
    } else showNotice(error.message, true);
  } finally { setBusy(false); }
}

// api отправляет запрос и переводит сетевые и серверные ошибки в понятные сообщения.
async function api(path, method = 'GET', body) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 20000);
  try {
    const response = await fetch(path, {
      method, signal: controller.signal,
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (response.status === 204) return null;
    const result = await response.json();
    if (!response.ok) throw new Error(result.error?.message || 'Не удалось выполнить запрос.');
    return result;
  } catch (error) {
    if (error.name === 'AbortError') throw new Error('Сервер не ответил вовремя. Обновите список перед повторной попыткой.');
    if (error instanceof TypeError) throw new Error('Нет связи с сервером. Проверьте соединение и обновите список.');
    if (error instanceof SyntaxError) throw new Error('Сервер вернул некорректный ответ. Попробуйте обновить список.');
    throw error;
  } finally { clearTimeout(timer); }
}

// filterQuery собирает применяемые фильтры и проверяет порядок дат.
function filterQuery() {
  const from = element('date-from').value;
  const to = element('date-to').value;
  if (from && to && from > to) throw new Error('Начало периода не может быть позже окончания.');
  const query = new URLSearchParams();
  if (element('filter-category').value) query.set('category_id', element('filter-category').value);
  if (from) query.set('date_from', from);
  if (to) query.set('date_to', to);
  return query.toString();
}

// reload обновляет справочник и расходы одной согласованной версией данных.
async function reload() {
  const query = filterQuery();
  try {
    const [categories, list] = await Promise.all([api('/api/categories'), api(`/api/expenses?${query}`)]);
    for (const record of [...categories, ...list.expenses]) {
      if (!Number.isSafeInteger(record.id) || record.id <= 0) throw new Error('Идентификатор записи слишком велик для этого интерфейса.');
    }
    list.expenses.forEach(expense => { formatAmount(expense.amount_kopecks); });
    formatAmount(list.total_kopecks);
    state.categories = categories;
    state.expenses = list.expenses;
    state.total = list.total_kopecks;
    state.loaded = true;
    render();
  } catch (error) {
    state.loaded = false;
    state.expenses = [];
    state.total = null;
    render();
    throw error;
  }
}

// actionButton создаёт кнопку действия без вставки пользовательского HTML.
function actionButton(label, action, id, danger = false) {
  const button = document.createElement('button');
  button.type = 'button';
  button.textContent = label;
  button.className = danger ? 'quiet danger' : 'quiet';
  button.dataset.action = action;
  button.dataset.id = String(id);
  return button;
}

// categoryOptions обновляет варианты категорий, сохраняя выбранное значение.
function categoryOptions(select, all = false) {
  const selected = select.value;
  select.replaceChildren();
  if (all) select.add(new Option('Все категории', ''));
  state.categories.forEach(category => select.add(new Option(category.name, String(category.id))));
  if ([...select.options].some(option => option.value === selected)) select.value = selected;
}

// render отображает категории, расходы и сумму, используя только текстовые DOM-операции.
function render() {
  categoryOptions(element('filter-category'), true);
  categoryOptions(element('expense-category'));
  element('category-hint').hidden = state.categories.length > 0;
  element('total').textContent = state.total === null ? '—' : formatAmount(state.total);
  element('expense-count').textContent = state.loaded ? `Записей: ${state.expenses.length}` : 'Не удалось загрузить данные';
  element('empty-expenses').hidden = !state.loaded || state.expenses.length > 0;
  element('expenses-table').hidden = state.expenses.length === 0;
  const rows = element('expense-rows');
  rows.replaceChildren();
  for (const expense of state.expenses) {
    const row = document.createElement('tr');
    const date = document.createElement('td');
    date.textContent = expense.date.split('-').reverse().join('.');
    const category = document.createElement('td');
    category.textContent = state.categories.find(item => item.id === expense.category_id)?.name || 'Категория недоступна';
    if (expense.comment) {
      const comment = document.createElement('span');
      comment.className = 'comment';
      comment.textContent = expense.comment;
      category.append(comment);
    }
    const amount = document.createElement('td');
    amount.className = 'amount';
    amount.textContent = formatAmount(expense.amount_kopecks);
    const actions = document.createElement('td');
    const buttons = document.createElement('div');
    buttons.className = 'row-actions';
    buttons.append(actionButton('Изменить', 'edit-expense', expense.id), actionButton('Удалить', 'delete-expense', expense.id, true));
    actions.append(buttons);
    row.append(date, category, amount, actions);
    rows.append(row);
  }
  const list = element('category-list');
  list.replaceChildren();
  element('empty-categories').hidden = !state.loaded || state.categories.length > 0;
  for (const category of state.categories) {
    const item = document.createElement('li');
    const name = document.createElement('span');
    name.textContent = category.name;
    const buttons = document.createElement('div');
    buttons.className = 'row-actions';
    buttons.append(actionButton('Изменить', 'edit-category', category.id), actionButton('Удалить', 'delete-category', category.id, true));
    item.append(name, buttons);
    list.append(item);
  }
}

// switchPanel переключает раздел без потери выбранных фильтров.
function switchPanel(panel) {
  for (const name of ['expenses', 'categories']) {
    element(`${name}-panel`).hidden = name !== panel;
    element(`${name}-tab`).setAttribute('aria-pressed', String(name === panel));
  }
}

// openExpense заполняет форму нового расхода или выбранной записи.
function openExpense(expense) {
  state.expenseID = expense?.id ?? null;
  element('expense-form').reset();
  categoryOptions(element('expense-category'));
  element('expense-title').textContent = expense ? 'Редактирование расхода' : 'Новый расход';
  const today = new Date();
  element('expense-date').value = expense?.date ?? `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;
  element('expense-amount').value = expense ? amountInput(expense.amount_kopecks) : '';
  element('expense-comment').value = expense?.comment ?? '';
  if (expense) element('expense-category').value = String(expense.category_id);
  element('expense-error').hidden = true;
  element('expense-dialog').showModal();
}

// openCategory открывает создание или переименование категории.
function openCategory(category) {
  state.categoryID = category?.id ?? null;
  element('category-title').textContent = category ? 'Редактирование категории' : 'Новая категория';
  element('category-name').value = category?.name ?? '';
  element('category-error').hidden = true;
  element('category-dialog').showModal();
}

// saveExpense сохраняет форму и обновляет список после успешного ответа.
async function saveExpense(event) {
  event.preventDefault();
  await perform(async () => {
    const body = {
      category_id: Number(element('expense-category').value),
      amount_kopecks: parseAmount(element('expense-amount').value),
      date: element('expense-date').value,
      comment: element('expense-comment').value.trim(),
    };
    await api(state.expenseID === null ? '/api/expenses' : `/api/expenses/${state.expenseID}`, state.expenseID === null ? 'POST' : 'PUT', body);
    element('expense-dialog').close();
    await reload();
    showNotice('Расход сохранён. Список показан с учётом выбранных фильтров.');
  }, 'expense-error');
}

// saveCategory сохраняет название и обновляет справочник во всех формах.
async function saveCategory(event) {
  event.preventDefault();
  await perform(async () => {
    const name = element('category-name').value.trim();
    if (!name) throw new Error('Укажите название категории.');
    await api(state.categoryID === null ? '/api/categories' : `/api/categories/${state.categoryID}`, state.categoryID === null ? 'POST' : 'PUT', { name });
    element('category-dialog').close();
    await reload();
    showNotice('Категория сохранена.');
  }, 'category-error');
}

// handleAction обрабатывает редактирование и подтверждённое удаление записи.
async function handleAction(event) {
  const button = event.target.closest('[data-action]');
  if (!button || state.busy) return;
  const id = Number(button.dataset.id);
  const action = button.dataset.action;
  if (action === 'edit-expense') return openExpense(state.expenses.find(expense => expense.id === id));
  if (action === 'edit-category') return openCategory(state.categories.find(category => category.id === id));
  const isExpense = action === 'delete-expense';
  if (!window.confirm(isExpense ? 'Удалить этот расход?' : 'Удалить категорию? Категорию с расходами удалить нельзя.')) return;
  await perform(async () => {
    await api(`/api/${isExpense ? 'expenses' : 'categories'}/${id}`, 'DELETE');
    if (!isExpense && element('filter-category').value === String(id)) element('filter-category').value = '';
    await reload();
    showNotice(isExpense ? 'Расход удалён.' : 'Категория удалена.');
  });
}

// initialize связывает элементы интерфейса и выполняет первоначальную загрузку.
function initialize() {
  element('expenses-tab').addEventListener('click', () => switchPanel('expenses'));
  element('categories-tab').addEventListener('click', () => switchPanel('categories'));
  element('add-expense').addEventListener('click', () => openExpense());
  element('add-category').addEventListener('click', () => openCategory());
  element('expense-form').addEventListener('submit', saveExpense);
  element('category-form').addEventListener('submit', saveCategory);
  element('filters').addEventListener('submit', event => { event.preventDefault(); perform(reload); });
  element('reset-filters').addEventListener('click', () => { element('filters').reset(); perform(reload); });
  element('refresh').addEventListener('click', () => perform(reload));
  document.addEventListener('click', handleAction);
  document.querySelectorAll('[data-close]').forEach(button => button.addEventListener('click', () => element(button.dataset.close).close()));
  document.querySelectorAll('dialog').forEach(dialog => dialog.addEventListener('cancel', event => { if (state.busy) event.preventDefault(); }));
  perform(reload);
}

initialize();
