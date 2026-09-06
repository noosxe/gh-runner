import { test, expect } from '@playwright/test';

test.describe('Flow 02: Onboarding Wizard & Configuration', () => {
  test.beforeEach(async ({ page }) => {
    // Ensure we are logged in
    await page.goto('/login');
    if (page.url().includes('/login')) {
      await page.getByLabel('Username').fill('admin');
      await page.getByLabel('Password').fill('AdminPassword123!');
      await page.getByRole('button', { name: /Sign In/i }).click();
    }
  });

  test('walks through git provider, safeguards, initial pool and completes onboarding', async ({ page }) => {
    await page.goto('/onboarding');

    // If on Step 2 (Connect Git Provider)
    if (await page.getByText(/Step 2 of 5: Connect Git Provider/i).isVisible()) {
      // Connect Git Provider using Personal Access Token
      const tokenInput = page.getByLabel('Personal Access Token (PAT)');
      await tokenInput.fill('ghp_mock_token_abcdef1234567890');

      const nextBtn = page.getByRole('button', { name: /Next: Safeguards/i });
      await nextBtn.click();
    }

    // Step 3: Safeguards
    await expect(page.getByText(/Step 3 of 5: Global Scaling Safeguards/i)).toBeVisible();
    await page.getByRole('button', { name: /Next: Initial Pool/i }).click();

    // Step 4: Initial Pool Setup
    await expect(page.getByText(/Step 4 of 5: Initial Runner Pool Setup/i)).toBeVisible();
    const repoUrlInput = page.getByLabel('Repository / Organization URL');
    await repoUrlInput.fill('https://github.com/test-org/test-repo');

    await page.getByRole('button', { name: /Next: Review & Launch/i }).click();

    // Step 5: Review & Launch
    await expect(page.getByText(/Step 5 of 5: Review & Launch Supervisor/i)).toBeVisible();
    await page.getByRole('button', { name: /Confirm & Launch Supervisor/i }).click();

    // After launch, should navigate to dashboard
    await page.waitForURL('/');
    await expect(page.getByText('Runner Dashboard')).toBeVisible();
  });
});
