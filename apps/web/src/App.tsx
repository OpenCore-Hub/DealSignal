import { useI18n } from './i18n/index.js';

function App() {
  const { locale, setLocale, t } = useI18n();

  return (
    <div style={{ padding: '2rem', fontFamily: 'system-ui, sans-serif' }}>
      <header style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
        <h1>{t('app.name')}</h1>
        <label>
          <span style={{ marginRight: '0.5rem' }}>{t('app.language')}</span>
          <select
            value={locale}
            onChange={(event) => setLocale(event.target.value as typeof locale)}
          >
            <option value="en">{t('common.english')}</option>
            <option value="zh-CN">{t('common.chinese')}</option>
          </select>
        </label>
      </header>
      <p>{t('app.tagline')}</p>
    </div>
  );
}

export default App;
