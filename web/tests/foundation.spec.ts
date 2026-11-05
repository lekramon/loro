import {expect,test} from '@playwright/test';
test('unconfigured startup provides a setup entry point',async({page})=>{
  await page.goto('/'); await expect(page.getByText('Not configured yet',{exact:true})).toBeVisible();
  await page.getByRole('link',{name:'Explore setup'}).click();
  await expect(page.getByRole('heading',{name:'Set up your library'})).toBeVisible();
  await page.reload(); await expect(page.getByText('Foundation preview.',{exact:false})).toBeVisible();
});
test('unknown API routes return JSON',async({request})=>{
  const response=await request.get('/api/v1/missing'); expect(response.status()).toBe(404);
  expect((await response.json()).code).toBe('not-found');
});
