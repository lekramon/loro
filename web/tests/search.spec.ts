import {expect,test} from '@playwright/test';

test('live movie search uses the API, handles pagination, unknowns and stale results',async({page})=>{
 const calls:URL[]=[];
 await page.route('**/api/v1/metadata/search?**',route=>{const url=new URL(route.request().url());calls.push(url);return route.fulfill({json:{page:Number(url.searchParams.get('page')),totalPages:2,totalResults:21,cache:{hit:false,stale:url.searchParams.get('page')==='2'},results:[{tmdbId:27205,title:'Inception',year:2010,overview:'A real API result from a simulator.',posterUrl:''},{tmdbId:1,title:'Unknown film',year:null,overview:'',posterUrl:''}]}})});
 await page.goto('/preview');await page.getByRole('button',{name:'Ramon',exact:true}).click();await page.getByRole('navigation',{name:'Library navigation',exact:true}).getByRole('link',{name:'Search',exact:true}).click();await expect(page).toHaveURL(/\/preview#\/search$/);
 await expect(page.locator('.persistent-sidebar')).toBeVisible();await expect(page.locator('.header-brand,.demo-brand')).toHaveCount(0);
 await page.getByLabel('Movie title',{exact:true}).fill('Inception');await page.getByLabel('Year (optional)').fill('2010');
 await page.getByRole('button',{name:'Search',exact:true}).click();await expect(page.getByRole('heading',{name:'Inception',exact:true})).toBeVisible();await expect(page.getByText('Unknown year',{exact:true})).toBeVisible();
 expect(calls[0].searchParams.get('year')).toBe('2010');expect(calls[0].searchParams.get('language')).toBe('en-US');
 await page.getByRole('button',{name:'Next page'}).click();await expect(page.getByText('Showing cached results because TMDB is unavailable.')).toBeVisible();expect(calls[1].searchParams.get('page')).toBe('2');
 await page.getByLabel('Movie title',{exact:true}).fill('Changed title');await page.getByRole('button',{name:'Previous page'}).click();await expect(page.getByRole('heading',{name:'Inception',exact:true})).toBeVisible();expect(calls[2].searchParams.get('query')).toBe('Inception');expect(calls[2].searchParams.get('page')).toBe('1');
 await page.screenshot({path:'test-results/live-search-desktop.png',fullPage:true});
 await page.setViewportSize({width:390,height:844});await expect(page.locator('.persistent-sidebar')).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await page.screenshot({path:'test-results/live-search-mobile.png',fullPage:true});
 await page.route('**/api/v1/metadata/search?**',route=>route.fulfill({status:503,json:{code:'metadata-unconfigured',message:'Sanitized'}}));await page.getByRole('button',{name:'Search',exact:true}).click();await expect(page.getByRole('alert')).toContainText('No TMDB token is configured');await expect(page.getByRole('heading',{name:'Inception',exact:true})).toHaveCount(0);
});
