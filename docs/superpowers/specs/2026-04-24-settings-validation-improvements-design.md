# Settings Page Validation & UX Improvements - Design Specification

**Date:** 2026-04-24  
**Project:** TwitchSMM Bot Farm  
**Status:** Approved for Implementation

---

## Overview

This specification addresses four identified issues in the Settings page:
1. Missing frontend validation for URL, API key, and rotation interval
2. Poor error handling in LLM connection testing (generic alerts)
3. Race condition with two save buttons
4. No confirmation before resetting changes

**Approach:** Minimal changes using existing patterns (Toast notifications, inline validation).

---

## Goals

- Add comprehensive frontend validation with inline error messages
- Replace alert() dialogs with Toast notifications for better UX
- Eliminate race condition by removing duplicate save button
- Add confirmation modal before destructive reset action
- Maintain existing code structure and styling

---

## Architecture

### New Files

```
frontend/src/
├── utils/
│   └── validation.ts          # Validation utility functions
├── components/
│   ├── Toast.tsx              # Reusable Toast notification component
│   └── ConfirmModal.tsx       # Confirmation modal component
```

### Modified Files

- `frontend/src/pages/Settings.tsx` - Main changes (validation, single save button, modal integration)

### Dependencies

No new npm packages required. Uses existing React, TypeScript, and @tanstack/react-query.

---

## Component Specifications

### 1. Validation Utility (`utils/validation.ts`)

**Purpose:** Centralized validation logic for all settings fields.

**Functions:**

#### `validateLLMUrl(url: string): string | null`
- **Validation level:** Moderate (allows localhost and IP addresses)
- **Rules:**
  - Non-empty string
  - Must start with `http://` or `https://`
  - Valid URL format (parseable by URL constructor)
  - Allows: localhost, 127.0.0.1, any domain
- **Returns:** Error message string or null if valid

#### `validateApiKey(key: string, provider: string): string | null`
- **Provider-specific validation:**
  - `lmstudio`: API key optional (returns null)
  - `openai` / `openrouter`: Must start with `sk-`, minimum 20 characters
  - Other providers: Non-empty check only
- **Returns:** Error message string or null if valid

#### `validateRotationInterval(interval: number): string | null`
- **Rules:**
  - Minimum: 10 minutes
  - Maximum: 1440 minutes (24 hours)
- **Returns:** Error message string or null if valid

#### `validateChannel(channel: string): string | null`
- **Rules:**
  - Empty channels allowed (filtered on save)
  - Non-empty: 4-25 characters, alphanumeric + underscore only
  - Matches Twitch username format: `/^[a-zA-Z0-9_]{4,25}$/`
- **Returns:** Error message string or null if valid

---

### 2. Toast Component (`components/Toast.tsx`)

**Purpose:** Reusable notification component for success, error, and info messages.

**Props:**
```typescript
interface ToastProps {
  type: 'success' | 'error' | 'info'
  title: string
  message: string
  onClose: () => void
}
```

**Behavior:**
- Auto-dismiss after 5 seconds
- Manual close via × button
- Slide-in animation from right (existing pattern)
- Fixed position: bottom-right corner
- z-index: 1000

**Styling:**
- Success: Green gradient (`linear-gradient(135deg, #10b981, #059669)`)
- Error: Red gradient (`linear-gradient(135deg, #ef4444, #dc2626)`)
- Info: Blue gradient (`linear-gradient(135deg, #3b82f6, #2563eb)`)
- Icons: ✓ (success), ✗ (error), ℹ (info)
- White text, rounded corners, shadow

---

### 3. ConfirmModal Component (`components/ConfirmModal.tsx`)

**Purpose:** Modal dialog for confirming destructive actions.

**Props:**
```typescript
interface ConfirmModalProps {
  isOpen: boolean
  title: string
  message: string
  confirmText?: string      // default: "Да"
  cancelText?: string       // default: "Отмена"
  onConfirm: () => void
  onCancel: () => void
  danger?: boolean          // red confirm button if true
}
```

**Behavior:**
- Renders only when `isOpen === true`
- Darkened backdrop (rgba(0, 0, 0, 0.5))
- Centered modal window
- Close on ESC key press
- Close on backdrop click
- Fade + scale animation on open

**Layout:**
- Title (bold, larger font)
- Message (normal text)
- Two buttons: Cancel (left, ghost style) and Confirm (right, primary or danger)

---

## Settings.tsx Changes

### New State Variables

```typescript
// Validation errors
const [urlError, setUrlError] = useState<string | null>(null)
const [apiKeyError, setApiKeyError] = useState<string | null>(null)
const [rotationIntervalError, setRotationIntervalError] = useState<string | null>(null)
const [channelErrors, setChannelErrors] = useState<Record<number, string>>({})

// Toast notification
const [toast, setToast] = useState<{
  type: 'success' | 'error' | 'info', 
  title: string, 
  message: string
} | null>(null)

// Reset confirmation modal
const [showResetConfirm, setShowResetConfirm] = useState(false)
```

### Validation on Field Change

**URL field:**
```typescript
const handleUrlChange = (value: string) => {
  setLlmApiUrl(value)
  const error = validateLLMUrl(value)
  setUrlError(error)
}
```

**API Key field:**
```typescript
const handleApiKeyChange = (value: string) => {
  setLlmApiKey(value)
  const error = validateApiKey(value, llmProvider)
  setApiKeyError(error)
}
```

**Provider dropdown:**
```typescript
const handleProviderChange = (provider: string) => {
  setLlmProvider(provider)
  
  // Auto-fill URL based on provider
  if (provider === 'lmstudio') {
    setLlmApiUrl('http://127.0.0.1:1234/v1')
  } else if (provider === 'openrouter') {
    setLlmApiUrl('https://openrouter.ai/api/v1')
  } else if (provider === 'openai') {
    setLlmApiUrl('https://api.openai.com/v1')
  }
  
  // Re-validate API key for new provider
  const error = validateApiKey(llmApiKey, provider)
  setApiKeyError(error)
}
```

**Rotation interval:**
```typescript
const handleRotationIntervalChange = (value: number) => {
  setRotationInterval(value)
  const error = validateRotationInterval(value)
  setRotationIntervalError(error)
}
```

**Channel inputs:**
```typescript
const updateChannel = (index: number, value: string) => {
  const newChannels = [...channels]
  newChannels[index] = value
  setChannels(newChannels)
  
  const error = validateChannel(value)
  if (error) {
    setChannelErrors({...channelErrors, [index]: error})
  } else {
    const newErrors = {...channelErrors}
    delete newErrors[index]
    setChannelErrors(newErrors)
  }
}
```

### Improved testLLMConnection

**Before:** Uses `alert()` with generic messages.

**After:** 
- Validates URL and API key before testing
- Shows Toast with detailed error messages
- Parses HTTP error codes (401 → "Неверный API ключ", 404 → "Endpoint не найден", etc.)
- Extracts error message from backend response

```typescript
const testLLMConnection = async () => {
  // Pre-test validation
  const urlErr = validateLLMUrl(llmApiUrl)
  const keyErr = validateApiKey(llmApiKey, llmProvider)
  
  if (urlErr || keyErr) {
    setToast({
      type: 'error',
      title: 'Ошибка валидации',
      message: urlErr || keyErr || 'Проверьте настройки'
    })
    return
  }
  
  setTestingConnection(true)
  try {
    const response = await api.getLLMStatus()
    if (response.data.available) {
      setToast({
        type: 'success',
        title: 'Подключение успешно!',
        message: `Провайдер ${response.data.provider} доступен`
      })
    } else {
      setToast({
        type: 'error',
        title: 'Подключение не удалось',
        message: response.data.error || 'Неизвестная ошибка'
      })
    }
  } catch (error: any) {
    let errorMsg = 'Ошибка при проверке подключения'
    
    // Parse HTTP errors
    if (error.response?.status === 401) {
      errorMsg = 'Неверный API ключ'
    } else if (error.response?.status === 404) {
      errorMsg = 'Endpoint не найден'
    } else if (error.code === 'ECONNREFUSED') {
      errorMsg = 'Сервер недоступен'
    }
    
    setToast({
      type: 'error',
      title: 'Ошибка подключения',
      message: errorMsg
    })
  } finally {
    setTestingConnection(false)
  }
}
```

### Single Save Button

**Remove:**
- `saveLLMSettings()` function
- "💾 Сохранить настройки LLM" button from JSX

**Keep:**
- Single "Сохранить все настройки" button at bottom

**Updated handleSave:**
```typescript
const handleSave = () => {
  // Validate all fields
  const urlErr = validateLLMUrl(llmApiUrl)
  const keyErr = validateApiKey(llmApiKey, llmProvider)
  const intervalErr = rotationEnabled ? validateRotationInterval(rotationInterval) : null
  
  // Validate channels
  const channelErrs: Record<number, string> = {}
  channels.forEach((ch, idx) => {
    const err = validateChannel(ch)
    if (err) channelErrs[idx] = err
  })
  
  // If any errors, show them and abort save
  if (urlErr || keyErr || intervalErr || Object.keys(channelErrs).length > 0) {
    setUrlError(urlErr)
    setApiKeyError(keyErr)
    setRotationIntervalError(intervalErr)
    setChannelErrors(channelErrs)
    
    setToast({
      type: 'error',
      title: 'Ошибка валидации',
      message: 'Исправьте ошибки в полях перед сохранением'
    })
    return
  }
  
  // Filter empty channels
  const filteredChannels = channels.filter(ch => ch.trim() !== '')
  
  // Proceed with save
  updateSettingsMutation.mutate({
    id: 'default',
    channels: filteredChannels,
    debug_mode: debugMode,
    debug_channel: debugChannel,
    positive_prompt: positivePrompt,
    negative_prompt: negativePrompt,
    max_tokens: maxTokens,
    audio_enabled: audioEnabled,
    rotation_enabled: rotationEnabled,
    rotation_interval: rotationInterval,
    llm_provider: llmProvider,
    llm_api_url: llmApiUrl,
    llm_model: llmModel,
    llm_api_key: llmApiKey,
    updated_at: new Date().toISOString(),
  })
}
```

### Reset Confirmation

**Before:** "Сбросить изменения" button directly calls `window.location.reload()`

**After:** Opens confirmation modal

```typescript
// Button
<button 
  className="btn ghost" 
  onClick={() => setShowResetConfirm(true)}
>
  Сбросить изменения
</button>

// Modal (at end of JSX)
<ConfirmModal
  isOpen={showResetConfirm}
  title="Сбросить изменения?"
  message="Все несохраненные изменения будут потеряны. Страница перезагрузится."
  confirmText="Сбросить"
  cancelText="Отмена"
  danger={true}
  onConfirm={() => window.location.reload()}
  onCancel={() => setShowResetConfirm(false)}
/>
```

### Error Display in UI

**Pattern for all validated fields:**
```tsx
<input
  id="llm-api-url"
  type="text"
  value={llmApiUrl}
  onChange={(e) => handleUrlChange(e.target.value)}
  placeholder="http://127.0.0.1:1234/v1"
  className={urlError ? 'error' : ''}
/>
{urlError && (
  <p className="error-message" style={{ 
    color: '#ef4444', 
    fontSize: '0.875rem', 
    marginTop: '0.25rem' 
  }}>
    {urlError}
  </p>
)}
```

**Apply to:**
- LLM API URL field
- LLM API Key field
- Rotation interval field (when rotation enabled)
- Each channel input field

---

## Error Handling

### Validation Errors
- Shown inline below the field
- Red text (#ef4444)
- Cleared when field becomes valid
- Prevent save if any validation errors exist

### Network Errors
- Caught in try/catch blocks
- Displayed via Toast notifications
- Specific messages for common HTTP codes:
  - 401: "Неверный API ключ"
  - 404: "Endpoint не найден"
  - ECONNREFUSED: "Сервер недоступен"
  - Default: "Ошибка при проверке подключения"

### Save Errors
- Handled by existing `updateSettingsMutation.onError`
- Shows error Toast (already implemented)

---

## Testing Checklist

### Validation
- [ ] Empty URL shows error
- [ ] URL without protocol shows error
- [ ] Valid localhost URL passes (http://localhost:1234)
- [ ] Valid IP URL passes (http://127.0.0.1:1234)
- [ ] Valid domain URL passes (https://api.openai.com/v1)
- [ ] OpenAI/OpenRouter API key without "sk-" prefix shows error
- [ ] LM Studio allows empty API key
- [ ] Rotation interval < 10 shows error
- [ ] Rotation interval > 1440 shows error
- [ ] Channel with 3 characters shows error
- [ ] Channel with special characters shows error
- [ ] Valid channel (4-25 alphanumeric + underscore) passes

### LLM Connection Test
- [ ] Test with invalid URL shows validation error Toast
- [ ] Test with invalid API key shows validation error Toast
- [ ] Successful connection shows success Toast
- [ ] Failed connection shows error Toast with backend message
- [ ] 401 error shows "Неверный API ключ"
- [ ] Network error shows appropriate message

### Save Button
- [ ] Only one save button visible
- [ ] Save blocked if validation errors exist
- [ ] Save shows validation error Toast if fields invalid
- [ ] Successful save shows success Toast (existing)
- [ ] Failed save shows error Toast (existing)

### Reset Confirmation
- [ ] "Сбросить изменения" opens modal
- [ ] Modal shows title and message
- [ ] "Отмена" closes modal without reload
- [ ] "Сбросить" reloads page
- [ ] ESC key closes modal
- [ ] Backdrop click closes modal

### UI/UX
- [ ] Error messages appear below fields
- [ ] Error messages disappear when field becomes valid
- [ ] Toast auto-dismisses after 5 seconds
- [ ] Toast can be manually closed with × button
- [ ] Modal backdrop darkens screen
- [ ] Modal is centered and readable

---

## Implementation Order

1. Create `utils/validation.ts` with all validation functions
2. Create `components/Toast.tsx` component
3. Create `components/ConfirmModal.tsx` component
4. Update `Settings.tsx`:
   - Add new state variables
   - Add validation handlers
   - Update `testLLMConnection`
   - Update `handleSave`
   - Remove `saveLLMSettings` and duplicate button
   - Add reset confirmation modal
   - Add error display JSX for all validated fields
5. Test all validation scenarios
6. Test all user flows (save, test connection, reset)

---

## Success Criteria

- ✅ All four identified problems resolved
- ✅ No new npm dependencies added
- ✅ Existing code structure preserved
- ✅ Consistent with existing UI patterns (Toast, styling)
- ✅ All validation rules enforced
- ✅ Better error messages for users
- ✅ No race conditions between save operations
- ✅ Destructive actions require confirmation

---

## Notes

- Validation is client-side only; backend validation already exists and remains unchanged
- Toast component is reusable for future features
- ConfirmModal component is reusable for other destructive actions
- All text is in Russian to match existing UI
- Styling uses inline styles and existing CSS classes to avoid CSS conflicts
