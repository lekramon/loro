import {test,expect} from '@playwright/test';
import {mkdtemp,mkdir,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve,relative,isAbsolute} from 'node:path';
test('folder explorer selects a server directory and cancels without changing the other path',async({page})=>{
 const root=await mkdtemp(join(tmpdir(),'piratarr-folder-browser-'));
 try{const downloads=join(root,'downloads');await mkdir(downloads);await page.goto('/setup');await page.getByLabel('Download directory',{exact:true}).fill(root);await page.getByRole('button',{name:'Browse folders',exact:true}).first().click();const dialog=page.getByRole('dialog');await expect(dialog).toBeVisible();await dialog.getByRole('button',{name:'downloads',exact:true}).click();await dialog.getByRole('button',{name:'Use this folder',exact:true}).click();await expect(dialog).not.toBeVisible();expect(resolve(await page.getByLabel('Download directory',{exact:true}).inputValue())).toBe(resolve(downloads));await page.getByRole('button',{name:'Browse folders',exact:true}).last().click();await page.getByRole('dialog').getByRole('button',{name:'Cancel',exact:true}).click();await expect(page.getByLabel('Movie directory',{exact:true})).toHaveValue('');await page.evaluate(()=>window.scrollTo(0,0)); await page.screenshot({path:'test-results/cinematic-folders-desktop.png',fullPage:true});
 }finally{const cleanupPath=relative(tmpdir(),root);if(!cleanupPath||cleanupPath.startsWith('..')||isAbsolute(cleanupPath))throw new Error('Unexpected temporary root');await rm(root,{recursive:true,force:true})}
});
