from django import template

register = template.Library()

@register.filter(name='short_id')
def short_id(value):
    if isinstance(value, str) and len(value) > 8:
        return value[-8:]
    return value