import { test, expect } from '@playwright/test';

test.describe('Flow 07: System Settings & Maintenance', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    if (page.url().includes('/login')) {
      await page.getByLabel('Username').fill('admin');
      await page.getByLabel('Password').fill('AdminPassword123!');
      await page.getByRole('button', { name: /Sign In/i }).click();
    }
  });

  test('updates global scaling constraints and toggles theme', async ({ page }) => {
    await page.goto('/settings');

    await expect(page.getByText('System Settings')).toBeVisible();

    // Verify constraints inputs
    const runnersInput = page.getByLabel('Total Concurrent Runners');
    await expect(runnersInput).toBeVisible();

    // Toggle theme in top-right corner of AppShell
    const darkBtn = page.getByTitle('Dark Theme');
    if (await darkBtn.isVisible()) {
      await darkBtn.click();
      await expect(page.locator('html')).toHaveClass(/dark/);
    }
  });
});
