import"./DsnmJJEf.js";import{p as m,c as S,f as b,s as y,x as _,d as l,a as f,e as w,h as c,r as i,m as v,g,u as x,o as F}from"./BKFspo1J.js";import{s as E,r as C}from"./CARHnQ8P.js";import{I as M}from"./CM27iA0o.js";import{i as O}from"./COK9M2N0.js";import{a as R}from"./DwMHXN8q.js";import{g as I}from"./CiHBgu9W.js";import{t as T}from"./CcBokjUl.js";import{H as q}from"./CQojf8Qs.js";function W(r,e){m(e,!0);let s=C(e,["$$slots","$$events","$$legacy"]);const o=[["rect",{width:"8",height:"4",x:"8",y:"2",rx:"1",ry:"1"}],["path",{d:"M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"}],["path",{d:"M12 11h4"}],["path",{d:"M12 16h4"}],["path",{d:"M8 11h.01"}],["path",{d:"M8 16h.01"}]];M(r,E({name:"clipboard-list"},()=>s,{get iconNode(){return o},children:(n,p)=>{var a=S(),t=b(a);y(t,()=>e.children??_),l(n,a)},$$slots:{default:!0}})),f()}function A(r,e){m(e,!0);let s=C(e,["$$slots","$$events","$$legacy"]);const o=[["path",{d:"m16 18 6-6-6-6"}],["path",{d:"m8 6-6 6 6 6"}]];M(r,E({name:"code"},()=>s,{get iconNode(){return o},children:(n,p)=>{var a=S(),t=b(a);y(t,()=>e.children??_),l(n,a)},$$slots:{default:!0}})),f()}var G=w('<div class="mb-4 w-full max-w-xl text-left"><p class="mb-2 text-xs text-muted-foreground">Example usage:</p> <div><!></div></div>'),P=w(`<div class="flex flex-col items-center justify-center py-8 text-center"><div class="mb-4 rounded-full bg-muted p-3"><!></div> <h3 class="mb-2 text-lg font-semibold">No Spans Recorded</h3> <p class="mb-4 max-w-md text-sm text-muted-foreground">Spans allow you to track the timing of individual operations within a transaction, such as
		database queries, HTTP calls, or cache operations.</p> <!></div>`);function z(r,e){m(e,!0);function s(d){switch(d){case"gin":return`// In your Gin handler
func MyHandler(c *gin.Context) {
    // Start a span for database operation
    span := traceway.StartSpan(c, "db.query")
    defer span.End()

    // Your database operation here
    result, err := db.Query("SELECT * FROM users")

    // Another span for cache
    cacheSpan := traceway.StartSpan(c, "cache.set")
    cache.Set("users", result)
    cacheSpan.End()
}`;case"fiber":return`// In your Fiber handler
func MyHandler(c *fiber.Ctx) error {
    // Start a span for database operation
    span := traceway.StartSpan(c.UserContext(), "db.query")
    defer span.End()

    // Your database operation here
    result, err := db.Query("SELECT * FROM users")

    // Another span for cache
    cacheSpan := traceway.StartSpan(c.UserContext(), "cache.set")
    cache.Set("users", result)
    cacheSpan.End()

    return c.JSON(result)
}`;default:return`span := traceway.StartSpan(ctx, "db.load")
// perform your operations here
span.End()`}}const o=["gin","fiber","chi","fasthttp","stdlib","custom"],n=x(()=>o.includes(e.framework)),p=x(()=>s(e.framework));var a=P(),t=c(a),$=c(t);A($,{class:"h-6 w-6 text-muted-foreground"}),i(t);var k=v(t,6);{var H=d=>{var u=G(),h=v(c(u),2),N=c(h);q(N,{get language(){return I},get code(){return g(p)}}),i(h),i(u),F(()=>R(h,1,`overflow-hidden rounded-md text-sm ${T.isDark?"dark-code":"light-code"}`)),l(d,u)};O(k,d=>{g(n)&&d(H)})}i(a),l(r,a),f()}export{W as C,z as S};
