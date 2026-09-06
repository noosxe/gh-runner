import { test, expect } from '@playwright/test';

test.describe('Flow 06: Renovate Bot Management', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    if (page.url().includes('/login')) {
      await page.getByLabel('Username').fill('admin');
      await page.getByLabel('Password').fill('AdminPassword123!');
      await page.getByRole('button', { name: /Sign In/i }).click();
    }
  });

  test('navigates to renovate dashboard and displays pool automation status', async ({ page }) => {
    await page.goto('/renovate');

    await expect(page.getByText('Renovate Bot Dashboard')).toBeVisible();
    await expect(page.getByText('Configured Pools')).toBeVisible();
    await expect(page.getByText('Renovate Active')).toBeVisible();
  });
});
