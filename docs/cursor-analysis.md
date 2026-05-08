# Cursor Behavior Analysis - TwitchSMM

## Executive Summary

The custom TargetCursor component globally hides the default cursor with `document.body.style.cursor = 'none'`, which creates cursor visibility issues across the application. Current fixes are **incomplete** and several edge cases remain unaddressed.

## Current Implementation

### TargetCursor Component
**Location:** `frontend/src/components/ui/TargetCursor.tsx`

**Behavior:**
- Sets `document.body.style.cursor = 'none'` globally (line 63)
- Only restores cursor on component unmount (line 309)
- Provides custom animated cursor with corner brackets
- Disabled on mobile devices

### Current Fixes Applied

1. **Input/Textarea Fix** (`frontend/src/styles/main.css` lines 83-91)
```css
input,
textarea,
select {
  cursor: text !important;
}

select {
  cursor: pointer !important;
}
```

2. **Modal Backdrop Fix** (`frontend/src/components/BotLogsModal.tsx` line 205)
```tsx
style={{ cursor: 'default' }}
```

## Issues Identified

### 1. **Incomplete Modal Coverage**

**BotLogsModal** has partial fix:
- ✅ Backdrop has `cursor: default` inline style
- ❌ Close button (line 361-370) lacks cursor override
- ❌ Interactive elements inside modal cards don't override cursor

**BotFormModal** has NO cursor fixes:
- ❌ Overlay (`.bot-modal-overlay`) has no cursor style
- ❌ Close button (`.bot-modal__close`) relies on CSS `cursor: pointer` which is overridden by global `cursor: none`
- ❌ Form inputs inside modal may show text cursor but buttons won't show pointer

### 2. **Button Cursor Issues**

All buttons across the app are affected:
- `.btn` class has `cursor: pointer` (main.css line 460)
- **Problem:** Global `cursor: none` overrides this
- **Affected components:**
  - Dock navigation buttons (have `.cursor-target` class)
  - Bot card action buttons (InteractiveHoverButton)
  - Settings page buttons
  - Form submit buttons
  - FloatingProviderStatus button

### 3. **Link Cursor Issues**

Navigation links and anchor tags:
- Settings page quick-tiles (lines 145-165) use `<a href="#">` with `.cursor-target`
- External links (e.g., Twitch dev console link) have no cursor override
- **Problem:** Links show no visual cursor feedback outside `.cursor-target` elements

### 4. **Interactive Elements Without `.cursor-target`**

Elements that should show pointer but don't:
- Checkbox rows (`.checkbox-row` - main.css line 388)
- Switch rows (`.switch-row` - main.css line 419)
- Debug panel header (main.css line 832)
- Floating panel elements
- Status bar elements with inline `cursor: 'pointer'`

### 5. **Dropdown/Select Elements**

- Select elements have `cursor: pointer !important` for the dropdown itself
- **Problem:** Dropdown options may not show proper cursor
- BotFormField select elements (BotFormField.css) may have issues

### 6. **Disabled State Confusion**

- Disabled buttons have `cursor: not-allowed` (main.css line 520)
- **Problem:** Global `cursor: none` hides this important UX feedback
- Users can't tell if button is disabled or just broken

## Edge Cases

### 1. **Scrollbar Interaction**
- Custom scrollbars in modals and panels
- No cursor override for scrollbar hover states

### 2. **Hover States**
- Elements with hover effects but no visible cursor
- Users can't tell what's interactive

### 3. **Loading States**
- InteractiveHoverButton shows loading spinner
- No cursor feedback during async operations

### 4. **Focus States**
- Keyboard navigation shows focus rings
- But mouse users see no cursor on focused elements

### 5. **Nested Modals/Overlays**
- Multiple overlay layers (modal over modal)
- Cursor behavior undefined in stacked contexts

## Architectural Issues

### 1. **Global State Pollution**
```tsx
document.body.style.cursor = 'none';  // Affects entire document
```
- Too aggressive
- Hard to override with CSS specificity
- Breaks browser defaults

### 2. **Inline Styles vs CSS Classes**
- Mix of inline `style={{ cursor: 'pointer' }}` and CSS classes
- Inline styles can't be overridden by global body style
- Inconsistent approach across codebase

### 3. **!important Overuse**
```css
cursor: text !important;
cursor: pointer !important;
```
- Necessary to fight global `cursor: none`
- Creates specificity wars
- Hard to maintain

## Recommended Solutions

### Option A: Scoped Cursor Management (Recommended)

**Approach:** Only hide cursor when hovering `.cursor-target` elements

```tsx
// TargetCursor.tsx - Modified approach
useEffect(() => {
  if (isMobile || !cursorRef.current) return;

  // DON'T set global cursor to none
  // Instead, add class to elements that should hide cursor
  
  const enterHandler = (e: Event) => {
    const target = e.target as Element;
    if (target.matches(targetSelector)) {
      target.classList.add('hide-default-cursor');
    }
  };
  
  const leaveHandler = (e: Event) => {
    const target = e.target as Element;
    if (target.matches(targetSelector)) {
      target.classList.remove('hide-default-cursor');
    }
  };
  
  window.addEventListener('mouseover', enterHandler);
  window.addEventListener('mouseout', leaveHandler);
  
  return () => {
    window.removeEventListener('mouseover', enterHandler);
    window.removeEventListener('mouseout', leaveHandler);
  };
}, [targetSelector, isMobile]);
```

```css
/* main.css */
.hide-default-cursor {
  cursor: none !important;
}

.hide-default-cursor * {
  cursor: none !important;
}
```

**Pros:**
- Surgical approach - only affects intended elements
- No need for !important overrides everywhere
- Browser defaults work normally
- Easy to debug

**Cons:**
- Requires refactoring TargetCursor component
- Need to test all `.cursor-target` elements

### Option B: CSS Class-Based System

**Approach:** Use CSS classes instead of inline styles

```css
/* main.css */
body.custom-cursor-active {
  cursor: none;
}

body.custom-cursor-active .cursor-default,
body.custom-cursor-active .modal-overlay,
body.custom-cursor-active .modal-backdrop {
  cursor: default !important;
}

body.custom-cursor-active .cursor-pointer,
body.custom-cursor-active button:not(:disabled),
body.custom-cursor-active a[href],
body.custom-cursor-active .clickable {
  cursor: pointer !important;
}

body.custom-cursor-active input,
body.custom-cursor-active textarea {
  cursor: text !important;
}

body.custom-cursor-active select {
  cursor: pointer !important;
}

body.custom-cursor-active button:disabled,
body.custom-cursor-active .disabled {
  cursor: not-allowed !important;
}
```

```tsx
// TargetCursor.tsx
useEffect(() => {
  if (isMobile) return;
  
  document.body.classList.add('custom-cursor-active');
  
  return () => {
    document.body.classList.remove('custom-cursor-active');
  };
}, [isMobile]);
```

**Pros:**
- Centralized cursor management
- Easy to add exceptions
- Clear override hierarchy

**Cons:**
- Still requires !important
- Need to identify all interactive elements

### Option C: Context-Based Cursor Management

**Approach:** Use React Context to manage cursor state

```tsx
// CursorContext.tsx
const CursorContext = createContext({
  hideCursor: false,
  setHideCursor: (hide: boolean) => {}
});

export function CursorProvider({ children }) {
  const [hideCursor, setHideCursor] = useState(false);
  
  useEffect(() => {
    document.body.style.cursor = hideCursor ? 'none' : '';
  }, [hideCursor]);
  
  return (
    <CursorContext.Provider value={{ hideCursor, setHideCursor }}>
      {children}
    </CursorContext.Provider>
  );
}

// Usage in modals
function Modal() {
  const { setHideCursor } = useCursor();
  
  useEffect(() => {
    setHideCursor(false); // Show cursor in modal
    return () => setHideCursor(true);
  }, []);
}
```

**Pros:**
- React-native solution
- Fine-grained control
- Easy to disable cursor per component

**Cons:**
- More complex
- Requires wrapping app in provider
- State management overhead

## Immediate Fixes Needed

### Priority 1: Critical UX Issues

1. **Fix all modal overlays**
```css
/* main.css - Add these rules */
.bot-modal-overlay,
.floating-panel-backdrop,
[class*="modal"][class*="overlay"],
[class*="backdrop"] {
  cursor: default !important;
}
```

2. **Fix all buttons**
```css
/* main.css - Add these rules */
button,
.btn,
[role="button"],
.cursor-target {
  cursor: pointer !important;
}

button:disabled,
.btn:disabled,
[aria-disabled="true"] {
  cursor: not-allowed !important;
}
```

3. **Fix all links**
```css
/* main.css - Add these rules */
a[href],
[role="link"] {
  cursor: pointer !important;
}
```

### Priority 2: Form Elements

```css
/* main.css - Enhance existing rules */
input:not([type="checkbox"]):not([type="radio"]),
textarea {
  cursor: text !important;
}

input[type="checkbox"],
input[type="radio"],
select,
.checkbox-row,
.switch-row {
  cursor: pointer !important;
}
```

### Priority 3: Interactive Components

```css
/* main.css - Add these rules */
.debug-messages-header,
.floating-panel-header,
.bot-card,
.quick-tile,
.stat-card,
[onclick],
[data-clickable="true"] {
  cursor: pointer !important;
}
```

## Testing Checklist

- [ ] Input fields show text cursor
- [ ] Textarea shows text cursor
- [ ] Select dropdowns show pointer cursor
- [ ] Buttons show pointer cursor
- [ ] Disabled buttons show not-allowed cursor
- [ ] Links show pointer cursor
- [ ] Modal overlays show default cursor
- [ ] Modal close buttons show pointer cursor
- [ ] Dock navigation buttons show pointer cursor
- [ ] Bot card buttons show pointer cursor
- [ ] Settings page buttons show pointer cursor
- [ ] Checkbox/radio inputs show pointer cursor
- [ ] Interactive tiles show pointer cursor
- [ ] Scrollbars show default cursor
- [ ] External links show pointer cursor
- [ ] Loading states show appropriate cursor

## Recommended Implementation Plan

### Phase 1: Immediate Hotfix (1-2 hours)
1. Add comprehensive CSS rules to `main.css` (Priority 1-3 fixes above)
2. Test all interactive elements
3. Deploy to dev environment

### Phase 2: Architectural Refactor (4-6 hours)
1. Implement Option A (Scoped Cursor Management)
2. Remove global `cursor: none` from body
3. Update TargetCursor component
4. Remove !important overrides
5. Test thoroughly

### Phase 3: Polish (2-3 hours)
1. Add cursor feedback for loading states
2. Improve disabled state visibility
3. Add hover state documentation
4. Create cursor behavior style guide

## Files Requiring Changes

### Immediate Fixes
- `frontend/src/styles/main.css` - Add comprehensive cursor rules

### Refactor (Option A)
- `frontend/src/components/ui/TargetCursor.tsx` - Scoped cursor logic
- `frontend/src/styles/main.css` - Add `.hide-default-cursor` class
- Remove inline `cursor` styles from:
  - `frontend/src/components/BotLogsModal.tsx`
  - `frontend/src/components/FloatingProviderStatus.tsx`
  - `frontend/src/components/StatusBar.tsx`
  - `frontend/src/components/SettingsLogPanel.tsx`
  - `frontend/src/components/BotLogPanel.tsx`

## Conclusion

Current fixes are **insufficient**. The global `cursor: none` approach is fundamentally flawed and creates a whack-a-mole situation where every interactive element needs explicit overrides.

**Recommended Action:** Implement Option A (Scoped Cursor Management) for long-term maintainability, with immediate Priority 1-3 CSS fixes as a stopgap.

**Estimated Impact:**
- Immediate fixes: 90% of cursor issues resolved
- Full refactor: 100% of cursor issues resolved + improved maintainability
