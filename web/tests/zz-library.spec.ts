import {expect,test} from '@playwright/test';
import {mkdtemp,mkdir,writeFile,readFile,unlink,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';

test('real inventory scans in place, marks missing and survives reload in both locales',async({page,request})=>{
 const root=await mkdtemp(join(tmpdir(),'piratarr-library-browser-'));
 try{
  const downloads=join(root,'downloads'),movies=join(root,'movies');await mkdir(downloads);await mkdir(movies);await writeFile(join(movies,'Example (2024).mkv'),'original movie fixture');await writeFile(join(movies,'Other.mp4'),'second fixture');await writeFile(join(movies,'note.txt'),'ignore text');
  const setup=await(await request.get('/api/v1/setup')).json();const input={directories:{downloadDirectory:downloads,movieDirectory:movies},profile:{acceptedResolutions:['1080p'],preferredResolution:'1080p',acceptedFormats:['remux','bluray','web-dl','webrip','hdtv','dvd'],acceptedContainers:['mkv','mp4','avi','mov','m4v','ts'],audioPreferences:['original']}};
  const current=setup.completed?await(await request.get('/api/v1/settings')).json():undefined;
  const response=await request.fetch(setup.completed?'/api/v1/settings':'/api/v1/setup',{method:setup.completed?'PUT':'POST',headers:{'X-Piratarr-Request':'1'},data:current?{...input,revision:current.revision}:input});expect(response.ok()).toBe(true);
  await page.goto('/preview#/files');await expect(page.getByRole('heading',{name:'Files',exact:true})).toBeVisible();await expect(page.getByText('No successful scan yet.',{exact:true})).toBeVisible();await page.getByRole('button',{name:'Scan folder',exact:true}).click();await expect(page.getByText('Example (2024).mkv',{exact:true})).toBeVisible();await expect(page.locator('.inventory-list li')).toHaveCount(2);expect(await readFile(join(movies,'Example (2024).mkv'),'utf8')).toBe('original movie fixture');const present=page.locator('.inventory-list li').filter({hasText:'Example (2024).mkv'});await present.getByRole('button',{name:'Ignore',exact:true}).click();await expect(present).toContainText('Ignored');await present.getByRole('button',{name:'Reopen',exact:true}).click();await expect(present).toContainText('Needs review');
  await unlink(join(movies,'Other.mp4'));await page.getByRole('button',{name:'Scan folder',exact:true}).click();await page.getByRole('combobox',{name:'Show files',exact:true}).selectOption('missing');await expect(page.locator('.inventory-list li')).toHaveCount(1);await expect(page.locator('.inventory-list')).toContainText('Other.mp4');await expect(page.locator('.file-state')).toHaveText('Missing');
  await page.reload();await expect(page.locator('.inventory-list li')).toHaveCount(2);await page.getByRole('button',{name:'Dark',exact:true}).click();await page.screenshot({path:'test-results/library-files-dark.png',fullPage:true});
  await page.setViewportSize({width:390,height:844});await page.getByLabel('Language',{exact:true}).selectOption('pt-BR');await expect(page.getByRole('heading',{name:'Arquivos',exact:true})).toBeVisible();await expect(page.locator('.file-state.missing')).toHaveText('Ausente');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await page.screenshot({path:'test-results/library-files-mobile.png',fullPage:true});
 }finally{if(root.startsWith(join(tmpdir(),'piratarr-library-browser-')))await rm(root,{recursive:true,force:true})}
});
