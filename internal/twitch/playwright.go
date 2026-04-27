package twitch

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

type PlaywrightClient struct {
	login    string
	password string
	channel  string

	mu        sync.Mutex
	pw        *playwright.Playwright
	browser   playwright.Browser
	page      playwright.Page
	connected bool
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewPlaywrightClient(login, password, channel string) *PlaywrightClient {
	return &PlaywrightClient{
		login:    login,
		password: password,
		channel:  channel,
	}
}

func (c *PlaywrightClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	log.Printf("[Playwright %s] Initializing browser...", c.login)

	// Инициализация Playwright с установкой драйверов
	runOptions := &playwright.RunOptions{
		SkipInstallBrowsers: false,
		Verbose:             true,
	}
	pw, err := playwright.Run(runOptions)
	if err != nil {
		return fmt.Errorf("failed to start playwright: %w", err)
	}
	c.pw = pw

	log.Printf("[Playwright %s] Launching browser...", c.login)

	// Запуск браузера в headless режиме
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
		},
	})
	if err != nil {
		pw.Stop()
		return fmt.Errorf("failed to launch browser: %w", err)
	}
	c.browser = browser

	// Создание контекста браузера с реалистичными параметрами
	browserContext, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		Viewport: &playwright.Size{
			Width:  1920,
			Height: 1080,
		},
		Locale:            playwright.String("ru-RU"),
		TimezoneId:        playwright.String("Europe/Moscow"),
		HasTouch:          playwright.Bool(false),
		IsMobile:          playwright.Bool(false),
		JavaScriptEnabled: playwright.Bool(true),
	})
	if err != nil {
		browser.Close()
		pw.Stop()
		return fmt.Errorf("failed to create context: %w", err)
	}

	// Создание страницы
	page, err := browserContext.NewPage()
	if err != nil {
		browser.Close()
		pw.Stop()
		return fmt.Errorf("failed to create page: %w", err)
	}
	c.page = page

	log.Printf("[Playwright %s] Logging in to Twitch...", c.login)

	// Переход на страницу логина
	if _, err := page.Goto("https://www.twitch.tv/login", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		c.closeLocked()
		return fmt.Errorf("failed to navigate to login: %w", err)
	}

	// Ожидание загрузки формы логина
	time.Sleep(time.Duration(1000+rand.Intn(1000)) * time.Millisecond)

	// Ввод логина
	if err := page.Fill("input[autocomplete='username']", c.login); err != nil {
		c.closeLocked()
		return fmt.Errorf("failed to fill username: %w", err)
	}
	time.Sleep(time.Duration(300+rand.Intn(500)) * time.Millisecond)

	// Ввод пароля
	if err := page.Fill("input[type='password']", c.password); err != nil {
		c.closeLocked()
		return fmt.Errorf("failed to fill password: %w", err)
	}
	time.Sleep(time.Duration(300+rand.Intn(500)) * time.Millisecond)

	// Нажатие кнопки входа
	if err := page.Click("button[type='submit']"); err != nil {
		c.closeLocked()
		return fmt.Errorf("failed to click login button: %w", err)
	}

	// Ожидание успешного входа (проверка редиректа или элемента)
	log.Printf("[Playwright %s] Waiting for login to complete...", c.login)
	if err := page.WaitForURL("https://www.twitch.tv/**", playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(30000),
	}); err != nil {
		c.closeLocked()
		return fmt.Errorf("login failed or timed out: %w", err)
	}

	time.Sleep(time.Duration(2000+rand.Intn(2000)) * time.Millisecond)

	log.Printf("[Playwright %s] Navigating to channel: %s", c.login, c.channel)

	// Переход на канал
	channelURL := fmt.Sprintf("https://www.twitch.tv/%s", c.channel)
	if _, err := page.Goto(channelURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		c.closeLocked()
		return fmt.Errorf("failed to navigate to channel: %w", err)
	}

	time.Sleep(time.Duration(2000+rand.Intn(2000)) * time.Millisecond)

	log.Printf("[Playwright %s] Successfully connected to channel %s", c.login, c.channel)

	c.connected = true
	c.ctx, c.cancel = context.WithCancel(context.Background())

	return nil
}

func (c *PlaywrightClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *PlaywrightClient) SendMessage(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.page == nil {
		return fmt.Errorf("not connected")
	}

	log.Printf("[Playwright %s] Sending message to %s: %s", c.login, c.channel, text)

	// Находим поле ввода чата
	chatInput := "textarea[data-a-target='chat-input']"

	// Кликаем на поле ввода
	if err := c.page.Click(chatInput); err != nil {
		return fmt.Errorf("failed to click chat input: %w", err)
	}
	time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)

	// Вводим текст с эмуляцией печати
	if err := c.page.Fill(chatInput, text); err != nil {
		return fmt.Errorf("failed to fill message: %w", err)
	}
	time.Sleep(time.Duration(300+rand.Intn(500)) * time.Millisecond)

	// Нажимаем Enter для отправки
	if err := c.page.Press(chatInput, "Enter"); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	log.Printf("[Playwright %s] Message sent successfully", c.login)

	return nil
}

func (c *PlaywrightClient) Reconnect() error {
	c.Close()
	time.Sleep(3 * time.Second)
	return c.Connect()
}

func (c *PlaywrightClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
}

func (c *PlaywrightClient) closeLocked() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	if c.page != nil {
		_ = c.page.Close()
		c.page = nil
	}

	if c.browser != nil {
		_ = c.browser.Close()
		c.browser = nil
	}

	if c.pw != nil {
		_ = c.pw.Stop()
		c.pw = nil
	}

	c.connected = false
	log.Printf("[Playwright %s] Disconnected", c.login)
}
