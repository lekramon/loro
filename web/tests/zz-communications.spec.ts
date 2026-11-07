import { test, expect } from '@playwright/test';

test('communication drawer polls only while open and supports pause, filters, locales and narrow layouts', async ({ page }) => {
 let calls = 0;
 const filters: string[] = [];
 await page.route('**/api/v1/system/communications?**', async route => {
  calls++; filters.push(new URL(route.request().url()).searchParams.get('integration') ?? '');
  await route.fulfill({ json: { sessionId:'1111111111111111', cursor:`1111111111111111:${calls}`, reset:false, truncated:false, storageAvailable:false, entries:[{id:`1111111111111111-${calls}`, timestamp:'2026-11-07T03:00:00Z',requestId:'22222222222222222222222222222222',integration:'browser',operation:'browser.search.document',phase:'browser.response',level:'warning',status:403,elapsedMs:1250,retryAfterSeconds:null,code:'source-challenge-required',count:null}] } });
 });
 await page.goto('/setup');
 const logs = page.getByRole('button', {name:'Logs',exact:true});
 await expect(logs).toBeVisible(); expect(calls).toBe(0);
 await logs.click(); const panel=page.locator('#communication-console');await expect(panel).toHaveAccessibleName('Communication logs');
 await expect(panel).toContainText('Browser response'); await expect(panel).toContainText('403');
 await expect(panel).toContainText('Log storage unavailable');
 await panel.getByRole('button',{name:'Pause logs',exact:true}).click(); const paused=calls;
 await page.waitForTimeout(1700);expect(calls).toBe(paused);
 await panel.getByRole('combobox',{name:'Integration',exact:true}).selectOption('browser');
 await panel.getByRole('button',{name:'Resume logs',exact:true}).click();await expect.poll(()=>filters.at(-1)).toBe('browser');
 await panel.getByText('Browser response',{exact:false}).first().click(); await expect(panel).toContainText('22222222222222222222222222222222');
 await page.getByRole('button',{name:'Dark',exact:true}).click();
 await page.screenshot({path:'test-results/communications-dark.png',fullPage:true});
 await page.getByRole('button',{name:'Português (Brasil)',exact:true}).click();
 await expect(panel).toContainText('Resposta do navegador');
 await page.setViewportSize({width:390,height:844});
 await page.getByRole('button',{name:'Claro',exact:true}).click();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 const box=await panel.boundingBox();expect(box).not.toBeNull();expect(box!.x).toBeGreaterThanOrEqual(60);expect(box!.x+box!.width).toBeLessThanOrEqual(390);
 await page.screenshot({path:'test-results/communications-mobile.png',fullPage:true});
 await panel.press('Escape');await expect(panel).not.toBeVisible();const closed=calls;await page.waitForTimeout(1700);expect(calls).toBe(closed);
 await page.goto('/preview#/profiles');await expect(page.locator('.communication-toggle')).toBeVisible();
});
