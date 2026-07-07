from django import template
from datetime import datetime

register = template.Library()

@register.filter(name='human_readable_datetime')
def human_readable_datetime(value):
    try:
        value_date = datetime.fromisoformat(value)
        return value_date.strftime("%B %d, %Y, %-I:%M %p %Z")
    except Exception as e:
        return value # lowk just return the original value if it can't be parsed