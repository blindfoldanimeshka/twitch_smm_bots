# Cursor Fix Implementation Guide

## Overview

This guide provides step-by-step instructions to fix cursor behavior issues in the TwitchSMM application. Two approaches are provided:

1. **Quick Fix** (30 minutes) - Add CSS overrides to fix immediate issues
2. **Proper Fix** (2-3 hours) - Refactor TargetCursor component for long-term solution

## Quick Fix - Immediate CSS Overrides

### Step 1: Backup Current Files

```bash
cd C:\Users\blindfold\Desktop\2026\twitchSmm
cp frontend/src/styles/main.css frontend/src/styles/main.css.backup
```

### Step 2: Add CSS Rules to main.css

Open `frontend/src/styles/main.css` and add the following at the end of the file (before the closing comment):

```css
/* ═══════════════════════════════════════════════════════════════════════
   CURSOR FIXES - Override global cursor: none
   ═══════════════════════════════════════════════════════════════════════ */

/* Modal overlays - default cursor */
.bot-modal-overlay,
.floating-panel-backdrop,
[class*="modal"][class*="overlay"],
[class*="backdrop"],
.fixed.inset-0 {
  cursor: default !important;
}

/* All buttons - pointer cursor */
button,
.btn,
[role="button"],
.cursor-target,
.bot-modal__close,
.floating-panel-button {
  cursor: pointer !important;
}

/* Disabled buttons - not-allowed cursor */
button:disabled,
.btn:disabled,
button[disabled],
[aria-disabled="true"] {
  cursor: not-allowed !important;
}

/* All links - pointer cursor */
a[href],
[role="link"],
.quick-tile {
  cursor: pointer !important;
}

/* Interactive components */
.debug-messages-header,
.floating-panel-header,
.checkbox-row,
.switch-row,
[onclick] {
  cursor: pointer !important;
}

/* Modal specific fixes */
.bot-modal button,
.bot-modal [role="button"] {
  cursor: pointer !important;
}

.bot-modal input:not([type="checkbox"]):not([type="radio"]),
.bot-modal textarea {
  cursor: text !important;
}
```

### Step 3: Test the Changes

```bash
cd frontend
npm run dev
```

Open browser and test:
- ✅ Click all buttons (should show pointer)
- ✅ Hover over links (should show pointer)
- ✅ Click in input fields (should show text cursor)
- ✅ Open modals and test close buttons
- ✅ Test disabled buttons (should show not-allowed)

### Step 4: Verify in Production Build

```bash
npm run build
npm run preview
```

**Pros of Quick Fix:**
- Fast implementation
- No component refactoring needed
- Works immediately

**Cons of Quick Fix:**
- Uses !important everywhere (maintenance burden)
- Doesn't fix root cause
- May need updates as new components are added

---

## Proper Fix - Refactor TargetCursor Component

### Step 1: Backup Current Files

```bash
cd C:\Users\blindfold\Desktop\2026\twitchSmm
cp frontend/src/components/ui/TargetCursor.tsx frontend/src/components/ui/TargetCursor.tsx.backup
cp frontend/src/components/ui/TargetCursor.css frontend/src/components/ui/TargetCursor.css.backup
cp frontend/src/styles/main.css frontend/src/styles/main.css.backup
```

### Step 2: Replace TargetCursor.tsx

Replace the content of `frontend/src/components/ui/TargetCursor.tsx` with the refactored version:

```bash
# Copy from the refactored file
cp C:\Users\blindfold\Desktop\2026\twitchSmm\TargetCursor-refactored.tsx frontend/src/components/ui/TargetCursor.tsx
```

Or manually copy the content from `TargetCursor-refactored.tsx`.

### Step 3: Update TargetCursor.css

Add the following to `frontend/src/components/ui/TargetCursor.css` at the end:

```css
/* Scoped cursor hiding - only on .cursor-target elements */
.hide-default-cursor {
  cursor: none !important;
}

.hide-default-cursor * {
  cursor: none !important;
}
```

### Step 4: Clean Up main.css

Remove the temporary cursor fixes from `main.css`:

**Keep these rules** (they're legitimate):
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

**Remove these rules** (no longer needed):
- Any rules with `cursor: pointer !important` on buttons
- Any rules with `cursor: default !important` on modals
- Any other cursor override rules added as quick fixes

### Step 5: Remove Inline Cursor Styles (Optional Cleanup)

These inline styles are no longer needed and can be removed:

**BotLogsModal.tsx** (line 205):
```tsx
// REMOVE: style={{ cursor: 'default' }}
<motion.div
  onClick={onClose}
  className="fixed inset-0 flex items-center justify-center z-[1000]"
  // Remove the cursor style prop
>
```

**FloatingProviderStatus.tsx** (line 78):
```tsx
// REMOVE: cursor: 'pointer',
style={{
  position: 'fixed',
  bottom: '32px',
  right: '32px',
  // ... other styles
  // Remove cursor: 'pointer'
}}
```

Similar cleanup in:
- `StatusBar.tsx`
- `SettingsLogPanel.tsx`
- `BotLogPanel.tsx`

### Step 6: Test Thoroughly

```bash
cd frontend
npm run dev
```

**Test Checklist:**

1. **Cursor Target Elements**
   - [ ] Dock navigation buttons show custom cursor
   - [ ] Quick tiles on Settings page show custom cursor
   - [ ] Interactive hover buttons show custom cursor
   - [ ] Custom cursor animates correctly on hover

2. **Non-Target Elements**
   - [ ] Regular buttons show pointer cursor
   - [ ] Links show pointer cursor
   - [ ] Input fields show text cursor
   - [ ] Disabled buttons show not-allowed cursor

3. **Modals**
   - [ ] Modal overlays show default cursor
   - [ ] Modal close buttons show pointer cursor
   - [ ] Form inputs in modals show text cursor
   - [ ] Buttons in modals show pointer cursor

4. **Edge Cases**
   - [ ] Checkboxes show pointer cursor
   - [ ] Select dropdowns show pointer cursor
   - [ ] Scrollbars show default cursor
   - [ ] Loading states show appropriate cursor

### Step 7: Verify Production Build

```bash
npm run build
npm run preview
```

Test the same checklist in production build.

### Step 8: Commit Changes

```bash
git add frontend/src/components/ui/TargetCursor.tsx
git add frontend/src/components/ui/TargetCursor.css
git add frontend/src/styles/main.css
git commit -m "refactor: scope TargetCursor to only hide cursor on target elements

- Remove global cursor: none from document.body
- Add .hide-default-cursor class applied only to .cursor-target elements
- Remove !important cursor overrides from main.css
- Improve UX by preserving browser default cursors everywhere else
- Fix cursor visibility issues in modals, buttons, and form elements"
```

**Pros of Proper Fix:**
- Fixes root cause
- No !important needed
- Better maintainability
- Proper separation of concerns
- Browser defaults work correctly

**Cons of Proper Fix:**
- Requires more testing
- Need to refactor component
- Takes more time initially

---

## Troubleshooting

### Issue: Custom cursor not showing on .cursor-target elements

**Solution:** Check that the element has the `.cursor-target` class:
```tsx
<button className="cursor-target">Click me</button>
```

### Issue: Cursor still hidden globally

**Solution:** Make sure you're using the refactored TargetCursor.tsx that doesn't set `document.body.style.cursor = 'none'`.

### Issue: Cursor flickers between default and custom

**Solution:** Check for conflicting CSS rules. Use browser DevTools to inspect computed styles.

### Issue: Cursor not hiding on hover

**Solution:** Verify `.hide-default-cursor` class is being added to the element. Check browser console for errors.

### Issue: Buttons still don't show pointer cursor

**Solution:** 
1. Check if button has inline `style={{ cursor: 'none' }}`
2. Check CSS specificity - may need to add button-specific rule
3. Verify the refactored TargetCursor is actually being used

---

## Rollback Instructions

If something goes wrong, rollback to original files:

```bash
cd C:\Users\blindfold\Desktop\2026\twitchSmm

# Restore original files
cp frontend/src/components/ui/TargetCursor.tsx.backup frontend/src/components/ui/TargetCursor.tsx
cp frontend/src/components/ui/TargetCursor.css.backup frontend/src/components/ui/TargetCursor.css
cp frontend/src/styles/main.css.backup frontend/src/styles/main.css

# Restart dev server
cd frontend
npm run dev
```

---

## Recommended Approach

**For immediate deployment:** Use Quick Fix
**For long-term solution:** Use Proper Fix

**Best strategy:**
1. Apply Quick Fix now to resolve user complaints
2. Schedule Proper Fix for next sprint
3. Test Proper Fix thoroughly in dev environment
4. Deploy Proper Fix when confident

---

## Additional Resources

- **Analysis Document:** `cursor-analysis.md` - Detailed analysis of all issues
- **Quick Fix CSS:** `cursor-fixes-immediate.css` - Ready-to-use CSS rules
- **Refactored Component:** `TargetCursor-refactored.tsx` - New component code
- **Refactored CSS:** `TargetCursor-refactored.css` - New CSS code

---

## Questions?

If you encounter issues not covered in this guide:

1. Check browser console for errors
2. Use DevTools to inspect computed cursor styles
3. Verify `.hide-default-cursor` class is being applied correctly
4. Test in different browsers (Chrome, Firefox, Safari)
5. Check for conflicting CSS from third-party libraries

---

## Success Criteria

The fix is successful when:

- ✅ All buttons show pointer cursor on hover
- ✅ All links show pointer cursor on hover
- ✅ Input fields show text cursor
- ✅ Disabled buttons show not-allowed cursor
- ✅ Modal overlays show default cursor
- ✅ Custom cursor works on .cursor-target elements
- ✅ No cursor flickering or glitches
- ✅ Works in all major browsers
- ✅ Works in production build
- ✅ No console errors related to cursor
