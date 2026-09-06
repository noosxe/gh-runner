import { test, expect } from '@playwright/test';

test.describe('Flow 03: Dashboard & System Metrics', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    if (page.url().includes('/login')) {
      await page.getByLabel('Username').fill('admin');
      await page.getByLabel('Password').fill('AdminPassword123!');
      await page.getByRole('button', { name: /Sign In/i }).click();
    }
  });

  test('displays key metric summary cards and latency widgets', async ({ page }) => {
    await page.goto('/');

    // Validate page header
    await expect(page.getByText('Runner Dashboard')).toBeVisible();

    // Validate metrics cards
    await expect(page.getByText(/Total Jobs/i)).toBeVisible();
    await expect(page.getByText(/Success Rate/i)).toBeVisible();
    await expect(page.getByText(/Avg Queue Latency/i)).toBeVisible();
    await expect(page.getByText(/Avg Job Runtime/i)).toBeVisible();

    // Validate Pools Summary section
    await expect(page.getByText(/Runner Pools Overview/i)).toBeVisible();
  });
});
