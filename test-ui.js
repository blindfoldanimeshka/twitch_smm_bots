const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  const page = await context.newPage();

  console.log('🔍 Testing TwitchSMM UI at http://localhost:3000\n');

  try {
    // 1. Загрузка главной страницы
    console.log('1️⃣ Loading main page...');
    await page.goto('http://localhost:3000', { waitUntil: 'networkidle', timeout: 10000 });
    console.log('✅ Main page loaded');

    // 2. Проверка заголовка
    const title = await page.title();
    console.log(`📄 Page title: ${title}`);

    // 3. Проверка навигации
    console.log('\n2️⃣ Checking navigation...');
    const navLinks = await page.$$('nav a, [role="navigation"] a, header a');
    console.log(`✅ Found ${navLinks.length} navigation links`);

    // 4. Проверка Dashboard
    console.log('\n3️⃣ Checking Dashboard page...');
    const dashboardVisible = await page.isVisible('text=Dashboard').catch(() => false) || 
                             await page.isVisible('text=Дашборд').catch(() => false) ||
                             await page.isVisible('text=Статистика').catch(() => false);
    console.log(dashboardVisible ? '✅ Dashboard elements found' : '⚠️ Dashboard elements not found');

    // 5. Переход на страницу Bots
    console.log('\n4️⃣ Navigating to Bots page...');
    const botsLink = await page.$('a[href*="bots"], a:has-text("Боты"), a:has-text("Bots")');
    if (botsLink) {
      await botsLink.click();
      await page.waitForTimeout(1000);
      console.log('✅ Navigated to Bots page');
      
      // Проверка списка ботов
      const botCards = await page.$$('[class*="bot"], [class*="card"]');
      console.log(`📊 Found ${botCards.length} bot cards/elements`);
    } else {
      console.log('⚠️ Bots link not found');
    }

    // 6. Переход на Settings
    console.log('\n5️⃣ Navigating to Settings page...');
    const settingsLink = await page.$('a[href*="settings"], a:has-text("Настройки"), a:has-text("Settings")');
    if (settingsLink) {
      await settingsLink.click();
      await page.waitForTimeout(1000);
      console.log('✅ Navigated to Settings page');
      
      // Проверка полей настроек
      const inputs = await page.$$('input, textarea, select');
      console.log(`📝 Found ${inputs.length} input fields`);
    } else {
      console.log('⚠️ Settings link not found');
    }

    // 7. Проверка API запросов
    console.log('\n6️⃣ Checking API requests...');
    await page.goto('http://localhost:3000', { waitUntil: 'networkidle' });
    
    const apiRequests = [];
    page.on('request', request => {
      if (request.url().includes('localhost:8080/api')) {
        apiRequests.push(request.url());
      }
    });
    
    await page.waitForTimeout(2000);
    console.log(`✅ Captured ${apiRequests.length} API requests`);
    if (apiRequests.length > 0) {
      console.log('API endpoints called:');
      [...new Set(apiRequests)].forEach(url => {
        console.log(`  - ${url.replace('http://localhost:8080', '')}`);
      });
    }

    // 8. Проверка консольных ошибок
    console.log('\n7️⃣ Checking for console errors...');
    const errors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });
    
    await page.reload({ waitUntil: 'networkidle' });
    await page.waitForTimeout(2000);
    
    if (errors.length > 0) {
      console.log(`⚠️ Found ${errors.length} console errors:`);
      errors.forEach(err => console.log(`  - ${err}`));
    } else {
      console.log('✅ No console errors found');
    }

    // 9. Screenshot
    console.log('\n8️⃣ Taking screenshot...');
    await page.screenshot({ path: 'twitchsmm-screenshot.png', fullPage: true });
    console.log('✅ Screenshot saved to twitchsmm-screenshot.png');

    console.log('\n✅ UI Test completed successfully!');

  } catch (error) {
    console.error('\n❌ Test failed:', error.message);
    process.exit(1);
  } finally {
    await browser.close();
  }
})();
