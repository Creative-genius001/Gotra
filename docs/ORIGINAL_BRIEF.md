# Original Brief (chat log)

Preserved verbatim from the initial handoff. The platform was renamed from
**Goray** to **Gotra**; the `.docx` specs still use the old name in places.

> [12:26 PM, 6/20/2026] Ovie Osamagbe Ighosuakpo: so there is the auth flow i had to implement
> Goray_Authentication_Identity_Architecture_v1.docx — this
> Goray_Frontend_Product_Design_Bible_v3.docx — this is for the frontend design
> Goray_Backend_Architecture_Bible_v1.docx — this is the backend arch
> i want a monorepo actually
> specify gin for the api design o — cos i think AI selects fiber by default for some reason
> also tell Claude that the name of the platform should be called Gotra not Goray
> Gotra_AI_Debugging_Service_Architecture_Bible_v1.docx — this is the AI backend service spec that should be added to the already existing backend specs
> Gotra_AI_Experience_and_Debugging_Product_Bible_v1.docx — while this is the AI debugging frontend spec that should be added to the already existing frontend spec
> so basically the Auth and AI service are separate docs that should be added to the backend spec
> while the AI debugging spec is a separate doc that should be added to the main frontend spec

## Source specifications

| Doc | Domain |
| --- | --- |
| `Goray_Backend_Architecture_Bible_v1.docx` | Master backend architecture |
| `Goray_Authentication_Identity_Architecture_v1.docx` | Auth & identity (merge into backend) |
| `Gotra_AI_Debugging_Service_Architecture_Bible_v1.docx` | AI debugging service (merge into backend) |
| `Goray_Frontend_Product_Design_Bible_v3.docx` | Master frontend / design system |
| `Gotra_AI_Experience_and_Debugging_Product_Bible_v1.docx` | AI debugging UX (merge into frontend) |
